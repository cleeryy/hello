package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/handlers"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/monitor"
	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/storage"
	wshub "github.com/cleeryy/hello/internal/websocket"
	"github.com/cleeryy/hello/internal/wol"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		src := "devices.example.json"
		if len(os.Args) > 2 {
			src = os.Args[2]
		}
		count, err := seedDevices(seedFile(), src)
		if err != nil {
			slog.Error("seed failed", slog.Any("err", err))
			os.Exit(1)
		}
		slog.Info("seeded devices", slog.Int("count", count))
		return
	}
	if err := run(); err != nil {
		slog.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

// autoScan runs a LAN sweep on a cadence until ctx ends. A failed sweep is
// logged, never fatal: discovery is best-effort onboarding.
func autoScan(ctx context.Context, disc *discover.Scanner, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scanCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			hosts, err := disc.Scan(scanCtx)
			cancel()
			if err != nil {
				slog.Warn("auto-scan failed", slog.Any("err", err))
				continue
			}
			slog.Info("auto-scan finished", slog.Int("hosts", len(hosts)))
		}
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	store, err := storage.New(cfg.DevicesFile)
	if err != nil {
		return err
	}
	hist, err := history.NewWithCapacity(cfg.HistoryFile, cfg.HistoryCap)
	if err != nil {
		return err
	}
	schedStore, err := scheduler.NewStoreWithCap(cfg.SchedulesFile, cfg.SchedulesCap)
	if err != nil {
		return err
	}
	origins, err := cfg.ParseCORSOrigins()
	if err != nil {
		return err
	}
	proxies, err := cfg.ParseTrustedProxies()
	if err != nil {
		return err
	}
	ignore, err := discover.LoadIgnoreList(cfg.IgnoredFile)
	if err != nil {
		return err
	}
	probePorts, err := cfg.ParseProbePorts()
	if err != nil {
		return err
	}
	tcpPorts, err := cfg.ParsePingTCPPorts()
	if err != nil {
		return err
	}
	if len(tcpPorts) > 0 {
		ping.TCPPorts = tcpPorts
	}

	hub := wshub.NewHub(origins...)
	go hub.Run(ctx)

	sch := scheduler.New(schedStore,
		func(id string) (models.Device, error) {
			dev, err := store.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *dev, nil
		},
		func(sched models.Schedule, dev models.Device) (bool, string) {
			errMsg := ""
			success := true
			if err := wol.SendWOLPacket(dev.MAC, wol.BroadcastForIP(dev.IP, cfg.BroadcastIP)); err != nil {
				slog.Error("scheduled wol failed",
					slog.String("schedule", sched.ID), slog.String("broadcast", wol.BroadcastForIP(dev.IP, cfg.BroadcastIP)), slog.Any("err", err))
				errMsg = err.Error()
				success = false
			}
			if _, err := hist.Record(models.WakeEvent{
				DeviceID: dev.ID,
				MAC:      dev.MAC,
				Trigger:  models.TriggerSchedule,
				Success:  success,
				Error:    errMsg,
			}); err != nil {
				slog.Warn("history record failed", slog.Any("err", err))
			}
			if success {
				if err := store.RecordWake(dev.ID, time.Now().Unix()); err != nil {
					slog.Warn("wake counter update failed", slog.String("device", dev.ID), slog.Any("err", err))
				}
			}
			return success, errMsg
		})
	go sch.Start(ctx)

	mon := monitor.NewWithTimeout(store, cfg.MonitorInterval, time.Duration(cfg.PingTimeoutSec)*time.Second)
	mon.OnStatusChange = hub.Broadcast
	mon.Start(ctx)
	defer mon.Stop()

	disc := discover.New()
	disc.SetCooldown(time.Duration(cfg.DiscoverCooldownSec) * time.Second)
	disc.Concurrency = cfg.DiscoverConcurrency
	if len(probePorts) > 0 {
		disc.ProbePorts = probePorts
	}
	if !cfg.DiscoverResolve {
		disc.ResolveHostname = nil
	}
	if cfg.DiscoverIntervalSec > 0 {
		go autoScan(ctx, disc, time.Duration(cfg.DiscoverIntervalSec)*time.Second)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(proxies); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	handlers.New(cfg, store, hub).
		WithHistory(hist).
		WithSchedules(schedStore, sch).
		WithDiscover(disc).
		WithIgnoreList(ignore).
		WithMonitor(mon).
		Mount(r)
	handlers.RegisterDashboard(r)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
