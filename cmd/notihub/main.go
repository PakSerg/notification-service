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

func main() {
	notificationRepo := repository.NewMemoryRepository()
	notificationService := service.NewNotificationService(notificationRepo)
	handler := transport.NewHandler(notificationService)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler.Router(),
	}

	go func() {
		log.Println("starting server on", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("server stopped")
}
