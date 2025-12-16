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

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		logging.Fatalf("cannot create data dir %s: %v", cfg.DataDir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logging.Warnf("shutdown signal received")
		cancel()
	}()

	if err := app.Init(ctx, cfg); err != nil {
		logging.Warnf("cannot load data: %v", err)
	}

	handler := app.NewHTTPHandler()

	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelTimeout()
		_ = server.Shutdown(ctxTimeout)
	}()

	logging.Infof("starting server on %s (DATA_DIR=%s)", cfg.HTTPAddr, cfg.DataDir)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logging.Fatalf("server error: %v", err)
	}
}
