package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mrshabel/kron/core"
)

const (
	consumerGroup = "kron-consumer"
)

func main() {
	cluster := core.GetEnvOrPanic("CLUSTER")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	consumer, err := core.NewKronConsumer(&core.ConsumerConfig{Topic: core.GetClusterTopic(cluster), GroupID: consumerGroup, Logger: logger})
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
