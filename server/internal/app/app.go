package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/database"
	"misu/internal/httpapi"
	"misu/internal/openplatform"
	"misu/internal/secret"
	"misu/internal/service"
	"misu/internal/wechat"
)

type App struct {
	Cfg    *config.Config
	Log    *slog.Logger
	DB     *gorm.DB
	Server *httpapi.Server

	logs     *service.LogService
	callback *service.CallbackService
	runner   *service.JobRunner
	audits   *service.AuditService
	jobs     *service.JobService
}

func New(cfg *config.Config) (*App, error) {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	log := slog.New(handler)

	db, err := database.Open(cfg, log)
	if err != nil {
		return nil, err
	}
	box, err := secret.NewBox(cfg.SecretEncKey)
	if err != nil {
		return nil, fmt.Errorf("init encryption box: %w", err)
	}
	wx := wechat.New(cfg.WXMaxQPS, cfg.WXTimeout)
	logs := service.NewLogService(db, log)
	wx.SetOnCall(logs.RecordCall)

	tokens := service.NewTokenManager(cfg, db, box, wx, log)
	auths := service.NewAuthorizerService(cfg, db, box, wx, tokens, logs, log)
	templates := service.NewTemplateService(db, wx, tokens, logs, log)
	assets := service.NewAuditAssetService(db, log)
	audits := service.NewAuditService(db, wx, auths, assets, log)

	runner := service.NewJobRunner(cfg, db, logs, log)
	for _, executor := range service.NewExecutors(db, wx, auths, audits, log) {
		runner.Register(executor)
	}
	jobs := service.NewJobService(cfg, db, runner, logs, assets, auths, log)
	callback := service.NewCallbackService(cfg, db, tokens, auths, audits, logs, log)
	dashboard := service.NewDashboardService(db, tokens, cfg)
	settings := service.NewSettingsService(cfg, db, tokens, logs, log)
	openPlatform := openplatform.New(cfg)

	server, err := httpapi.New(cfg, log, db, logs, tokens, auths, templates, assets, audits, jobs, callback, dashboard, settings, openPlatform, wx)
	if err != nil {
		return nil, err
	}
	return &App{
		Cfg: cfg, Log: log, DB: db, Server: server,
		logs: logs, callback: callback, runner: runner, audits: audits, jobs: jobs,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	appCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.logs.Start(appCtx)
	a.callback.Start(appCtx)
	a.runner.Start(appCtx)
	service.StartBackgroundWorkers(appCtx, a.Cfg, a.audits, a.jobs, a.Log)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", a.Cfg.ServerPort),
		Handler:           a.Server.Engine(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	errCh := make(chan error, 1)
	go func() {
		a.Log.Info("server started", "port", a.Cfg.ServerPort, "public_base_url", a.Cfg.PublicBaseURL, "version", a.Cfg.Version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		cancel()
		return err
	case <-ctx.Done():
	}

	a.Log.Info("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		a.Log.Warn("graceful shutdown failed", "error", err)
	}
	cancel()
	a.Log.Info("server stopped")
	return nil
}
