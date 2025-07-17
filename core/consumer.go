package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

const (
	DefaultConsumerShutdownTimeout = 10 * time.Second
	DefaultJobTimeout              = 30 * time.Second
	DefaultBrokerPollIntervalMs    = 100
)

type ConsumerConfig struct {
	Topics                  []string
	GroupID                 string
	Logger                  *slog.Logger
	BrokerPollIntervalMs    int
	ConsumerShutdownTimeout time.Duration
	JobTimeout              time.Duration
}

func (cfg *ConsumerConfig) validate() error {
	if len(cfg.Topics) == 0 {
		return fmt.Errorf("topic(s) is required")
	}
	if cfg.GroupID == "" {
		return fmt.Errorf("consumer group ID is required")
	}

	// set defaults
	if cfg.BrokerPollIntervalMs <= 0 {
		cfg.BrokerPollIntervalMs = DefaultBrokerPollIntervalMs
	}
	if cfg.ConsumerShutdownTimeout <= 0 {
		cfg.ConsumerShutdownTimeout = DefaultConsumerShutdownTimeout
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = DefaultJobTimeout
	}

	// add logger if not passed
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return nil
}

type Consumer struct {
	config   *ConsumerConfig
	Topics   []string
	GroupID  string
	consumer *kafka.Consumer
	logger   *slog.Logger
}

// NewKronConsumer creates a new instance of a kafka consumer for kron. The caller should call the [Shutdown] method when done
func NewKronConsumer(cfg *ConsumerConfig) (*Consumer, error) {
	// validate config and set defaults
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	consumer, err := NewConsumer(cfg.Topics, cfg.GroupID)
	if err != nil {
		return nil, err
	}

	// subscribe to topics
	if err = consumer.SubscribeTopics(cfg.Topics, nil); err != nil {
		return nil, err
	}

	return &Consumer{
		consumer: consumer,
		config:   cfg,
		Topics:   cfg.Topics,
		GroupID:  cfg.GroupID,
		logger:   cfg.Logger,
	}, nil
}

// Start runs the kron consumer and processes the tasks retrieved
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("Kron consumer started. Waiting for messages...")
	for {
		ev := c.consumer.Poll(c.config.BrokerPollIntervalMs)
		switch e := ev.(type) {
		case *kafka.Message:
			// parse message
			var job Job
			if err := json.Unmarshal(e.Value, &job); err != nil {
				c.logger.Error("failed to unmarshal message", "message", string(e.Value), "error", err)
				continue
			}

			// process on callback
			if err := c.RunJob(&job); err != nil {
				c.logger.Error("failed to process consumed job", "job_id", job.ID, "command", job.Command, "error", err)
				// TODO: retry job with backoff then push to retry topic
				continue
			}
			c.logger.Info("Job execution complete", "job_id", job.ID, "scheduled_at", job.ScheduledAt, "completed_at", time.Now().String())

			// commit
			if _, err := c.consumer.CommitMessage(e); err != nil {
				c.logger.Error("Failed to commit message offset", "job_id", job.ID, "partition", e.TopicPartition.Partition, "offset", e.TopicPartition.Offset)
			}
		case kafka.Error:
			if e.IsFatal() || !e.IsRetriable() {
				c.logger.Error("Fatal: Kafka error. Consumer may shutdown", "error", e.Error())
				return e
			}
			c.logger.Error("Kafka error", "error", e.Error())
		}
	}
}

// runJob executes the command specified in the job
func (c *Consumer) RunJob(job *Job) error {
	// TODO: sanitize job

	ctx, cancel := context.WithTimeout(context.Background(), c.config.JobTimeout)
	defer cancel()

	// run as shell script
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", job.Command)
	// optionally check for output
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if output != nil {
		c.logger.Info("Job executed with output", "job_id", job.ID, "command", job.Command, "output", string(output))
	}
	return nil
}

// Shutdown closes the running instance of the kafka consumer
func (c *Consumer) Shutdown() error {
	return c.consumer.Close()
}
