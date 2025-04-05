package kafka

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Producer interface {
	SendMessage(ctx context.Context, topic string, key []byte, value []byte) error
	Close() error
}

type ConsoleProducer struct{}

func NewConsoleProducer() Producer {
	log.Println("Initialized Console Kafka Producer (Placeholder)")
	return &ConsoleProducer{}
}

func (p *ConsoleProducer) SendMessage(ctx context.Context, topic string, key []byte, value []byte) error {
	select {
	case <-time.After(50 * time.Millisecond):

		fmt.Printf("\n--- KAFKA_PRODUCER (CONSOLE) ---\n")
		fmt.Printf("Topic: %s\n", topic)
		fmt.Printf("Key: %s\n", string(key))
		fmt.Printf("Value: %s\n", string(value))
		fmt.Printf("--- END KAFKA ---")
		return nil
	case <-ctx.Done():
		log.Printf("KAFKA_PRODUCER (CANCELLED): Topic=[%s], Key=[%s]", topic, string(key))
		return ctx.Err()
	}
}

func (p *ConsoleProducer) Close() error {
	log.Println("Closing Console Kafka Producer")
	return nil
}
