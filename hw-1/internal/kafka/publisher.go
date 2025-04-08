package kafka

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

type PublisherConfig struct {
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
}

type Publisher struct {
	db             db.DB
	repo           storage.OutboxTaskRepository
	producer       Producer
	config         PublisherConfig
	wg             sync.WaitGroup
	shutdownSignal chan struct{}
	stopOnce       sync.Once
}

func NewPublisher(db db.DB, repo storage.OutboxTaskRepository, producer Producer, config PublisherConfig) *Publisher {
	return &Publisher{
		db:             db,
		repo:           repo,
		producer:       producer,
		config:         config,
		shutdownSignal: make(chan struct{}),
	}
}

func (p *Publisher) Run(ctx context.Context) {
	log.Println("Starting Outbox Publisher...")
	p.wg.Add(1)
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				log.Printf("ERROR: Outbox Publisher failed to process batch: %v", err)
			}
		case <-p.shutdownSignal:
			log.Println("Outbox Publisher received shutdown signal, stopping...")
			return
		case <-ctx.Done():
			log.Println("Outbox Publisher context cancelled, stopping...")
			p.Shutdown()
			return
		}
	}
}

func (p *Publisher) Shutdown() {
	p.stopOnce.Do(func() {
		log.Println("Initiating Outbox Publisher shutdown...")
		close(p.shutdownSignal)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			log.Println("Outbox Publisher shutdown complete.")
		case <-shutdownCtx.Done():
			log.Println("WARN: Outbox Publisher shutdown timed out.")
		}

		// Close the producer
		if err := p.producer.Close(); err != nil {
			log.Printf("ERROR: Failed to close Kafka producer: %v", err)
		}
	})
}

func (p *Publisher) processBatch(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for fetching tasks: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	tasks, err := p.repo.GetProcessableTasks(ctx, p.db, p.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get processable tasks: %w", err)
	}

	if len(tasks) == 0 {
		return tx.Commit(ctx)
	}

	log.Printf("Outbox Publisher: Fetched %d tasks to process", len(tasks))

	taskIDs := make([]uuid.UUID, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
		err := p.repo.UpdateTaskStatusTx(ctx, tx, task.ID, repository.TaskStatusProcessing, task.Attempts, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to mark task %s as PROCESSING: %w", task.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction after marking tasks as PROCESSING: %w", err)
	}

	for _, task := range tasks {
		select {
		case <-p.shutdownSignal:
			log.Printf("Shutdown signal received during batch processing, task %s not processed.", task.ID)
			return errors.New("publisher shutdown during batch processing")
		case <-ctx.Done():
			log.Printf("Context cancelled during batch processing, task %s not processed.", task.ID)
			return ctx.Err()
		default:

		}

		err := p.processSingleTask(ctx, task)
		if err != nil {
			log.Printf("ERROR: Failed to process task %s: %v", task.ID, err)

		}
	}

	return nil
}

func (p *Publisher) processSingleTask(ctx context.Context, task *repository.OutboxTask) error {
	log.Printf("Processing task %s (Attempt %d)", task.ID, task.Attempts+1)

	kafkaKey := []byte(task.ID.String())

	err := p.producer.SendMessage(ctx, task.Topic, kafkaKey, task.Payload)

	if err != nil {
		log.Printf("Failed to send task %s to producer: %v", task.ID, err)
		newAttempts := task.Attempts + 1
		newStatus := repository.TaskStatusFailed
		errMsg := err.Error()

		if newAttempts >= p.config.MaxAttempts {
			log.Printf("Task %s reached max attempts (%d), marking as FAILED permanently.", task.ID, p.config.MaxAttempts)
		}

		updateErr := p.repo.UpdateTaskStatus(ctx, p.db, task.ID, newStatus, newAttempts, &errMsg, nil)
		if updateErr != nil {
			log.Printf("CRITICAL ERROR: Failed to update task %s status to %s after send failure: %v (Original send error: %v)", task.ID, newStatus, updateErr, err)
			return fmt.Errorf("failed to update task status after send failure: %w", updateErr)
		}
		return err
	}

	log.Printf("Task %s processed successfully.", task.ID)
	now := time.Now().UTC()
	updateErr := p.repo.UpdateTaskStatus(ctx, p.db, task.ID, repository.TaskStatusDone, task.Attempts, nil, &now)
	if updateErr != nil {
		log.Printf("ERROR: Failed to update task %s status to DONE after successful send: %v", task.ID, updateErr)
		return fmt.Errorf("failed to update task status after successful send: %w", updateErr)
	}

	return nil
}
