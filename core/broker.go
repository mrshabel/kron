// this file contains code for interacting with the kafka broker
package core

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

const (
	ClientID = "kron-service"
)

// NewProducer creates a new instance of a kafka producer
func NewProducer() (*kafka.Producer, error) {
	producer, err := kafka.NewProducer(
		&kafka.ConfigMap{
			"bootstrap.servers": config.Brokers,
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
func NewConsumer(topic string, groupID string) (*kafka.Consumer, error) {
	consumer, err := kafka.NewConsumer(
		&kafka.ConfigMap{
			"bootstrap.servers": config.Brokers,
			"group.id":          groupID,
			"auto.offset.reset": "smallest",
		},
	)
	if err != nil {
		return nil, err
	}
	if err = consumer.Subscribe(topic, nil); err != nil {
		return nil, err
	}
	return consumer, nil
}

// GetClusterTopic retrieves the topic name for a given cluster
func GetClusterTopic(cluster string) string {
	return fmt.Sprintf("jobs-%s", cluster)
}

// GetClusterRetryTopic retrieves the retry topic name for a given cluster
func GetClusterRetryTopic(cluster string) string {
	return fmt.Sprintf("jobs-retry-%s", cluster)
}

// GetClusterDLQ retrieves the dead-letter queue topic for a given cluster
func GetClusterDLQ(cluster string) string {
	return fmt.Sprintf("jobs-dlq-%s", cluster)
}
