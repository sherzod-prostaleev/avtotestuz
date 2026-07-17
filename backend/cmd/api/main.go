package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger, _ := zap.NewDevelopment()
	if cfg.Env == "prod" {
		logger, _ = zap.NewProduction()
	}
	defer func() { _ = logger.Sync() }()

	h := server.New(cfg, server.Deps{})
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("listening", zap.Int("port", cfg.Port), zap.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("stopped")
}
