# Makefile for distributed cron system
KAFKA_CONTAINER := kron-kafka
# default topics with retry and dlq
TOPICS := jobs-cluster-a jobs-cluster-b jobs-retry-cluster-a jobs-retry-cluster-b jobs-dlq-cluster-a jobs-dlq-cluster-b
# optional args
PARTITIONS ?= 2
REPLICATION ?= 1

.PHONY: build up up-kafka down clean compile setup-topics help

build: # Build Docker images for producer and consumer
	docker compose build

start: # Start all services (producer, consumers, Kafka)
	docker compose up

start-kafka: # Start Kafka service only
	docker compose up kafka -d

stop: # Stop all services
	docker compose down

clean: # Remove unused Docker images
	docker image prune --filter dangling=true

compile: # Compile producer and consumer binaries to bin/
	go build -o bin/producer cmd/producer/main.go
	go build -o bin/consumer cmd/consumer/main.go

setup-topics: # Create Kafka topics for clusters (jobs-cluster-a, jobs-cluster-b)
	@for topic in $(TOPICS); do \
		docker exec $(KAFKA_CONTAINER) kafka-topics.sh \
			--create \
			--if-not-exists \
			--bootstrap-server localhost:9092 \
			--topic $$  topic \
			--partitions $(PARTITIONS) \
			--replication-factor $(REPLICATION) || exit 1; \
	done

help: # Display available commands
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*#' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*#"}; {printf "  \033[36m%-15s\033[0m %s\n",   $$1, $$2}'