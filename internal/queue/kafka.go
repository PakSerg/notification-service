// Package queue carries delivery jobs between the API process and the
// worker process over Kafka: the API publishes a notification's ID after
// saving it, and the worker consumes it to perform the actual delivery.
// Decoupling the two lets the API return instantly regardless of how slow or
// unreliable the downstream channel is, and lets worker instances scale
// independently of API instances.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// commitTimeout bounds how long committing a processed job's offset may take,
// so a broker hiccup stalls Run instead of hanging it forever.
const commitTimeout = 10 * time.Second

// Producer publishes delivery jobs to a Kafka topic.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer builds a Producer writing to topic on the given brokers. It
// does not connect eagerly; the first Publish call establishes the
// connection.
func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:  kafka.TCP(brokers...),
			Topic: topic,
			// Every message for a given notification carries the same key
			// (its ID), so a producer's retries or any future multi-event
			// use of this topic keep per-notification ordering.
			Balancer:               &kafka.Hash{},
			RequiredAcks:           kafka.RequireAll,
			AllowAutoTopicCreation: true,
		},
	}
}

// Publish hands the notification identified by id to the queue for delivery.
func (p *Producer) Publish(ctx context.Context, id string) error {
	msg := kafka.Message{Key: []byte(id), Value: []byte(id)}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish notification %s: %w", id, err)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// Consumer reads delivery jobs off a Kafka topic as part of groupID, sharing
// the topic's partitions with any other consumer in the same group.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer builds a Consumer reading topic on the given brokers as part
// of groupID.
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			GroupID: groupID,
			Topic:   topic,
			// Offsets are committed explicitly in Run, only once a job has
			// actually been processed.
			CommitInterval: 0,
		}),
	}
}

// Run fetches jobs one at a time and passes each one's notification ID to
// handle, until ctx is canceled. A job's offset is committed only after
// handle returns successfully; if handle errors, the offset is left behind
// so the job is redelivered after this process restarts or the group
// rebalances - handle (typically NotificationService.Dispatch) is expected
// to be idempotent, so reprocessing it is safe.
//
// Once a job has been fetched, it is always run to completion on a context
// independent of ctx: canceling ctx (e.g. on SIGTERM) stops Run from
// fetching any further job, but does not abort whichever one is already in
// flight, so it can be committed rather than immediately redelivered.
func (c *Consumer) Run(ctx context.Context, handle func(ctx context.Context, id string) error) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		id := string(msg.Value)
		if err := handle(context.Background(), id); err != nil {
			log.Printf("process delivery job for notification %s: %v; left uncommitted for redelivery", id, err)
			continue
		}

		commitCtx, cancel := context.WithTimeout(context.Background(), commitTimeout)
		err = c.reader.CommitMessages(commitCtx, msg)
		cancel()
		if err != nil {
			log.Printf("commit offset for notification %s: %v", id, err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
