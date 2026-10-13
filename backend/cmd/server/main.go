package main

import (
	"context"
	"genco/finance/internal/api"
	"genco/finance/internal/config"
	"genco/finance/internal/store"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func main() {
	config.LoadDotEnv("backend/.env", ".env", "../.env")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dir := env("GENCO_DATA_DIR", "./data")
	db, err := store.Open(ctx, dir, os.Getenv("GENCO_MYSQL_DSN"))
	if err != nil {
		logger.Error("storage failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	s := &api.Server{Store: db, Dir: dir, BaseURL: env("GENCO_PUBLIC_URL", "http://localhost:8080"), Frontend: env("GENCO_FRONTEND_DIR", "../frontend/dist"), Log: logger}
	server := &http.Server{Addr: env("GENCO_ADDR", "127.0.0.1:8080"), Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); s.Worker(ctx) }()
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server listening", "address", server.Addr, "mysql", os.Getenv("GENCO_MYSQL_DSN") != "")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			serverErrors <- err
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	<-workerDone
	select {
	case <-serverErrors:
		_ = db.Close()
		os.Exit(1)
	default:
	}
}
