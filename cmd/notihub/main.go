package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PakSerg/NotiHub/internal/config"
	"github.com/PakSerg/NotiHub/internal/ratelimit"
	"github.com/PakSerg/NotiHub/internal/repository"
	"github.com/PakSerg/NotiHub/internal/service"
	"github.com/PakSerg/NotiHub/internal/transport"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	notificationRepo, err := repository.NewSQLiteRepository(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := notificationRepo.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	log.Printf("notihub %s, database %s", version, cfg.DBPath)

	var limiter transport.RateLimiter
	if cfg.RateLimitRPS > 0 {
		limiter = ratelimit.New(cfg.RateLimitRPS, cfg.RateLimitBurst)
		log.Printf("rate limiting enabled: %.1f req/s, burst %d", cfg.RateLimitRPS, cfg.RateLimitBurst)
	}

	notificationService := service.NewNotificationService(notificationRepo)
	handler := transport.NewHandler(notificationService, limiter)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Println("starting server on", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
	}

	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("server stopped")

	return nil
}
