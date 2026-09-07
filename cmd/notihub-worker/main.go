// Command notihub-worker consumes delivery jobs published by the notihub API
// process and performs the actual notification delivery. Running several
// instances under the same NOTIHUB_KAFKA_CONSUMER_GROUP splits the topic's
// partitions between them, scaling delivery throughput independently of the
// API.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/PakSerg/NotiHub/internal/config"
	"github.com/PakSerg/NotiHub/internal/provider"
	"github.com/PakSerg/NotiHub/internal/queue"
	"github.com/PakSerg/NotiHub/internal/repository"
	"github.com/PakSerg/NotiHub/internal/service"
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

	notificationRepo, err := repository.NewPostgresRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := notificationRepo.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	retryPolicy := service.RetryPolicy{
		MaxAttempts: cfg.RetryMaxAttempts,
		BaseDelay:   cfg.RetryBaseDelay,
		MaxDelay:    cfg.RetryMaxDelay,
	}
	notificationService := service.NewNotificationService(notificationRepo, provider.NewRegistry(), service.WithRetryPolicy(retryPolicy))

	consumer := queue.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaConsumerGroup)
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Printf("close kafka consumer: %v", err)
		}
	}()

	log.Printf("notihub-worker %s starting, topic %s, group %s", version, cfg.KafkaTopic, cfg.KafkaConsumerGroup)

	// Run blocks until ctx is canceled (SIGTERM/SIGINT), then returns nil:
	// whichever job is already in flight runs to completion first, but no
	// new one is fetched afterward.
	return consumer.Run(ctx, notificationService.Dispatch)
}
