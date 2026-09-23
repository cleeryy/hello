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

	mon := monitor.New(store, cfg.MonitorInterval)
	mon.OnStatusChange = hub.Broadcast
	mon.Start(ctx)
	defer mon.Stop()

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
		WithDiscover(discover.New()).
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
