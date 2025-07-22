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
	Topic                   string
	GroupID                 string
	Logger                  *slog.Logger
	BrokerPollIntervalMs    int
	ConsumerShutdownTimeout time.Duration
	JobTimeout              time.Duration
	BrokerDeliveryTimeout   time.Duration
}

func (cfg *ConsumerConfig) validate() error {
	if cfg.Topic == "" {
		return fmt.Errorf("topic is required")
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
	if cfg.BrokerDeliveryTimeout <= 0 {
		cfg.BrokerDeliveryTimeout = DefaultBrokerDeliveryTimeout
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
	Cluster  string
	Topic    string
	GroupID  string
	consumer *kafka.Consumer
	// producer instance for retries
	producer             *kafka.Producer
	producerDeliveryChan chan kafka.Event
	logger               *slog.Logger
}

// NewKronConsumer creates a new instance of a kafka consumer for kron. The caller should call the [Shutdown] method when done
func NewKronConsumer(cfg *ConsumerConfig) (*Consumer, error) {
	// validate config and set defaults
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	consumer, err := NewConsumer(cfg.Topic, cfg.GroupID)
	if err != nil {
		return nil, err
	}

	// subscribe to topics
	if err = consumer.Subscribe(cfg.Topic, nil); err != nil {
		return nil, err
	}
	producer, err := NewProducer()
	if err != nil {
		return nil, err
	}

	return &Consumer{
		consumer:             consumer,
		producer:             producer,
		producerDeliveryChan: make(chan kafka.Event, 1000),
		config:               cfg,
		Cluster:              config.Cluster,
		Topic:                cfg.Topic,
		GroupID:              cfg.GroupID,
		logger:               cfg.Logger,
	}, nil
}

// Start runs the kron consumer and processes the tasks retrieved
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("Kron consumer started. Waiting for messages...", "cluster", c.Cluster)
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
				c.logger.Error("failed to process consumed job", "jobId", job.ID, "command", job.Command, "error", err)
				if err := c.Retry(&job); err != nil {
					c.logger.Error("Fatal: Failed to add job to retry queue", "jobId", job.ID, "topic", e.TopicPartition.Topic, "partition", e.TopicPartition.Partition, "error", err)
				}
			} else {
				c.logger.Info("Job execution complete", "jobId", job.ID, "scheduledAt", job.ScheduledAt, "completedAt", time.Now().String())
			}

			// commit
			if _, err := c.consumer.CommitMessage(e); err != nil {
				c.logger.Error("Failed to commit message offset", "jobId", job.ID, "partition", e.TopicPartition.Partition, "offset", e.TopicPartition.Offset)
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
		c.logger.Info("Job executed with output", "jobId", job.ID, "command", job.Command, "output", string(output))
	}
	return nil
}

// Retry adds the given job to the retry topic
func (c *Consumer) Retry(job *Job) error {
	job.Retries++
	// add to dead letter queue
	if job.Retries >= job.MaxRetries {
		c.logger.Error("Max retries exceeded. Sending to dead-letter queue", "jobId", job.ID, "command", job.Command)
		return c.AddToDLQ(job)
	}

	return c.AddToRetry(job)
}

// AddToRetry adds the given job to the retry topic
func (c *Consumer) AddToRetry(job *Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}
	topic := GetClusterRetryTopic(c.Cluster)
	msg := &kafka.Message{
		Key:            []byte(job.ID),
		Value:          payload,
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
	}

	if err := c.producer.Produce(msg, c.producerDeliveryChan); err != nil {
		return err
	}

	// wait for delivery report
	select {
	case <-time.After(c.config.BrokerDeliveryTimeout):
		return ErrBrokerDeliveryTimeout
	case e := <-c.producerDeliveryChan:
		res := e.(*kafka.Message)
		if res.TopicPartition.Error != nil {
			return res.TopicPartition.Error
		}
		c.logger.Info("job added to retry queue", "jobId", job.ID, "topic", res.TopicPartition.Topic, "partition", res.TopicPartition.Partition)
	}

	return nil
}

// AddToDLQ adds the given job to the dead-letter queue
func (c *Consumer) AddToDLQ(job *Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}
	topic := GetClusterDLQ(c.Cluster)
	msg := &kafka.Message{
		Key:            []byte(job.ID),
		Value:          payload,
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
	}

	if err := c.producer.Produce(msg, c.producerDeliveryChan); err != nil {
		return err
	}

	// wait for delivery report
	select {
	case <-time.After(c.config.BrokerDeliveryTimeout):
		return ErrBrokerDeliveryTimeout
	case e := <-c.producerDeliveryChan:
		res := e.(*kafka.Message)
		if res.TopicPartition.Error != nil {
			return res.TopicPartition.Error
		}
		c.logger.Info("job added to dead letter queue", "jobId", job.ID, "topic", res.TopicPartition.Topic, "partition", res.TopicPartition.Partition)
	}

	return nil
}

// Shutdown closes the running instance of the kafka consumer
func (c *Consumer) Shutdown() error {
	c.producer.Close()
	return c.consumer.Close()
}
