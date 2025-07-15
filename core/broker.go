// this file contains code for interacting with the kafka broker
package core

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Config struct {
	Brokers string
}

const (
	ClientID = "kron-service"
)

var cfg = &Config{
	Brokers: "localhost:9092",
}

// NewProducer creates a new instance of a kafka producer
func NewProducer() (*kafka.Producer, error) {
	producer, err := kafka.NewProducer(
		&kafka.ConfigMap{
			"bootstrap.servers": cfg.Brokers,
			"client.id":         ClientID,
			"acks":              "all",
		},
	)

	if err != nil {
		return nil, err
	}
	return producer, nil
}

// NewConsumer creates a new instance of the kafka consumer. It is the caller's responsibility to close the consumer on shutdown
func NewConsumer(topics []string, groupID string) (*kafka.Consumer, error) {
	consumer, err := kafka.NewConsumer(
		&kafka.ConfigMap{
			"bootstrap.servers": cfg.Brokers,
			"group.id":          groupID,
			"auto.offset.reset": "smallest",
		},
	)
	if err != nil {
		return nil, err
	}
	if err = consumer.SubscribeTopics(topics, nil); err != nil {
		return nil, err
	}
	return consumer, nil
}
