package core

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// brokers comma-separated addresses
	Brokers string
	Cluster string
}

var config = &Config{
	Brokers: GetEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092"),
	Cluster: GetEnv("CLUSTER", "cluster-a"),
}

func GetEnv(key string, fallback string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	return fallback
}

func GetEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	v, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return v
}

func GetEnvOrPanic(key string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	panic(fmt.Errorf("%s is required", key))
}
