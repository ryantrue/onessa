package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ryantrue/onessa/app"
	"github.com/ryantrue/onessa/internal/logging"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	var cfg app.Config
	if err := env.Parse(&cfg); err != nil {
		logging.Fatalf("cannot parse env: %v", err)
	}

	logging.Init(logging.Options{Level: cfg.LogLevel, Format: cfg.LogFormat})

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		logging.Fatalf("cannot create data dir %s: %v", cfg.DataDir, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Init(ctx, cfg); err != nil {
		logging.Warnf("cannot load data: %v", err)
	}

	handler := app.NewHTTPHandler(cfg)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctxTimeout)
	}()

	logging.Infof("starting server on %s (DATA_DIR=%s STATIC_DIR=%s)", cfg.HTTPAddr, cfg.DataDir, cfg.StaticDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logging.Fatalf("server error: %v", err)
	}
}
