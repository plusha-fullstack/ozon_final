package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	kafkaBrokers = "localhost:9092"
	kafkaTopic   = "audit_logs"
	groupID      = "audit-log-consumer-group"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Println("Starting Kafka Consumer...")

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaBrokers},
		GroupID:        groupID,
		Topic:          kafkaTopic,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		MaxWait:        3 * time.Second,
	})
	defer func() {
		log.Println("Closing Kafka reader...")
		if err := r.Close(); err != nil {
			log.Printf("Error closing Kafka reader: %v", err)
		}
	}()

	log.Printf("Consumer connected to topic '%s' on brokers %s", kafkaTopic, kafkaBrokers)

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown signal received, stopping consumer.")
			return
		default:
			m, err := r.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					log.Println("Context cancelled, exiting message loop.")
					return
				}
				log.Printf("Error reading message: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			fmt.Printf("\n--- KAFKA_CONSUMER ---")
			fmt.Printf("Timestamp: %s\n", m.Time.Format(time.RFC3339))
			fmt.Printf("Partition: %d\n", m.Partition)
			fmt.Printf("Offset:    %d\n", m.Offset)
			fmt.Printf("Key:       %s\n", string(m.Key))
			fmt.Printf("Value:     %s\n", string(m.Value))
			fmt.Println("--- END CONSUMER ---")

		}
	}
}
