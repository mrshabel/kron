package main

import (
	"context"
	"log"

	"github.com/mrshabel/kron/core"
)

func main() {
	producer, err := core.NewKronProducer(&core.ProducerConfig{Topic: "jobs"})
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Shutdown()
	log.Println("Producer started successfully")

	if err := producer.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
