package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PakSerg/NotiHub/internal/repository"
	"github.com/PakSerg/NotiHub/internal/service"
	"github.com/PakSerg/NotiHub/internal/transport"
)

// defaultDBPath points at the project root, so running `go run ./cmd/notihub`
// creates notihub.db next to go.mod. Override it with NOTIHUB_DB_PATH.
const defaultDBPath = "notihub.db"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPath := defaultDBPath
	if path := os.Getenv("NOTIHUB_DB_PATH"); path != "" {
		dbPath = path
	}

	notificationRepo, err := repository.NewSQLiteRepository(ctx, dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := notificationRepo.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	log.Println("using database", dbPath)

	notificationService := service.NewNotificationService(notificationRepo)
	handler := transport.NewHandler(notificationService)

	server := &http.Server{
		Addr:              ":8080",
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("server stopped")

	return nil
}
