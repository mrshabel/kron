package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mrshabel/kron/core"
)

const (
	defaultConsumerGroup = "kron-consumer"
	retryConsumerGroup   = "kron-retry-consumer"
	dlqConsumerGroup     = "kron-dlq-consumer"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// compose topic and consumer group
	cluster := core.GetEnvOrPanic("CLUSTER")
	isRetryConsumer := core.GetEnvBool("RETRY", false)
	isDLQConsumer := core.GetEnvBool("DLQ", false)
	topic := core.GetClusterTopic(cluster)
	consumerGroup := defaultConsumerGroup

	if isRetryConsumer {
		topic = core.GetClusterRetryTopic(cluster)
		consumerGroup = retryConsumerGroup
	}
	if isDLQConsumer {
		topic = core.GetClusterDLQ(cluster)
		consumerGroup = dlqConsumerGroup
	}

	consumer, err := core.NewKronConsumer(&core.ConsumerConfig{Topic: topic, GroupID: consumerGroup, Logger: logger})
	if err != nil {
		logger.Error("Failed to start consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Shutdown()

	if err := consumer.Start(context.Background()); err != nil {
		logger.Error("Consumer encountered an error", "error", err)
		os.Exit(1)
	}
}
