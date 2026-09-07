package queue

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// testBrokers returns the Kafka bootstrap addresses to run against, skipping
// the test when none is configured (e.g. running `go test` locally without a
// broker). CI provides one via a service container.
func testBrokers(t *testing.T) []string {
	t.Helper()

	raw := os.Getenv("NOTIHUB_TEST_KAFKA_BROKERS")
	if raw == "" {
		t.Skip("NOTIHUB_TEST_KAFKA_BROKERS not set, skipping Kafka integration test")
	}
	return strings.Split(raw, ",")
}

func TestProducerConsumerRoundTrip(t *testing.T) {
	brokers := testBrokers(t)
	// A fresh topic and group per run keep this test independent of any
	// other test (or previous run) sharing the same broker.
	topic := "notihub-test-" + generateSuffix()
	group := "notihub-test-group-" + generateSuffix()

	producer := NewProducer(brokers, topic)
	t.Cleanup(func() {
		if err := producer.Close(); err != nil {
			t.Errorf("unexpected error on producer close: %v", err)
		}
	})

	consumer := NewConsumer(brokers, topic, group)
	t.Cleanup(func() {
		if err := consumer.Close(); err != nil {
			t.Errorf("unexpected error on consumer close: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const wantID = "abc123"

	received := make(chan string, 1)
	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = consumer.Run(ctx, func(_ context.Context, id string) error {
			received <- id
			return nil
		})
	}()

	publishCtx, publishCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer publishCancel()
	if err := producer.Publish(publishCtx, wantID); err != nil {
		t.Fatalf("unexpected error on publish: %v", err)
	}

	select {
	case got := <-received:
		if got != wantID {
			t.Fatalf("expected notification id %q, got %q", wantID, got)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for consumed message")
	}

	cancel()
	wg.Wait()
	if runErr != nil {
		t.Fatalf("unexpected error from Run: %v", runErr)
	}
}

func TestConsumerRunStopsOnContextCancel(t *testing.T) {
	brokers := testBrokers(t)
	topic := "notihub-test-" + generateSuffix()
	group := "notihub-test-group-" + generateSuffix()

	consumer := NewConsumer(brokers, topic, group)
	t.Cleanup(func() {
		if err := consumer.Close(); err != nil {
			t.Errorf("unexpected error on consumer close: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- consumer.Run(ctx, func(context.Context, string) error {
			return errors.New("should not be called: nothing was published")
		})
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected Run to return nil on context cancel, got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for Run to return after cancel")
	}
}

func generateSuffix() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
