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
	if cfg.APIToken == "" {
		slog.Warn("API_TOKEN unset: API runs open, set a token before exposing it")
	}

	store := storage.New(cfg.DevicesFile)

	hist, err := history.New(cfg.HistoryFile)
	if err != nil {
		return err
	}

	hub := wshub.NewHub()
	go hub.Run(ctx)

	schedStore := scheduler.NewStore(cfg.SchedulesFile)
	sch := scheduler.New(schedStore,
		func(id string) (models.Device, error) {
			dev, err := store.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *dev, nil
		},
		func(sched models.Schedule, dev models.Device) {
			errMsg := ""
			success := true
			if err := wol.SendWOLPacket(dev.MAC, cfg.BroadcastIP); err != nil {
				slog.Error("scheduled wol failed",
					slog.String("schedule", sched.ID), slog.Any("err", err))
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
		})
	go sch.Start(ctx)

	mon := monitor.New(store, cfg.MonitorInterval)
	mon.OnStatusChange = hub.Broadcast
	mon.Start(ctx)
	defer mon.Stop()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	handlers.New(cfg, store, hub).WithHistory(hist).WithSchedules(schedStore, sch).WithDiscover(discover.New()).Mount(r)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
