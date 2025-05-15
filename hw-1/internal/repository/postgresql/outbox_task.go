package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

type OutboxTaskRepo struct {
}

func NewOutboxTaskRepo() storage.OutboxTaskRepository {
	return &OutboxTaskRepo{}
}

func (r *OutboxTaskRepo) CreateTx(ctx context.Context, tx db.Tx, task *repository.OutboxTask) error {
	query := `
        INSERT INTO outbox_tasks (id, status, payload, topic, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, query,
		task.ID,
		repository.TaskStatusCreated,
		task.Payload,
		task.Topic,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox task: %w", err)
	}
	return nil
}

func (r *OutboxTaskRepo) GetProcessableTasks(ctx context.Context, db db.DB, limit int) ([]*repository.OutboxTask, error) {
	query := `
        SELECT id, status, payload, topic, attempts, last_error, created_at, updated_at, completed_at
        FROM outbox_tasks
        WHERE status = $1 OR (status = $2 AND attempts < $3) -- Add retry limit logic if needed
        ORDER BY updated_at ASC
        LIMIT $4
        FOR UPDATE SKIP LOCKED
    `
	maxAttempts := 5

	var tasks []*repository.OutboxTask
	err := db.Select(ctx, &tasks, query, repository.TaskStatusCreated, repository.TaskStatusFailed, maxAttempts, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get processable outbox tasks: %w", err)
	}
	return tasks, nil
}

func (r *OutboxTaskRepo) updateTaskStatusInternal(ctx context.Context, executor interface{}, id uuid.UUID, status repository.TaskStatus, attempts int, lastError *string, completedAt *time.Time) error {
	query := `
        UPDATE outbox_tasks
        SET
            status = $2,
            attempts = $3,
            last_error = $4,
            completed_at = $5
            -- updated_at is handled by the trigger
        WHERE id = $1
    `

	var cmdTag pgconn.CommandTag
	var err error

	switch exec := executor.(type) {
	case db.Tx:
		cmdTag, err = exec.Exec(ctx, query, id, status, attempts, lastError, completedAt)
	case db.DB:
		cmdTag, err = exec.Exec(ctx, query, id, status, attempts, lastError, completedAt)
	default:
		return fmt.Errorf("unsupported executor type: %T", executor)
	}

	if err != nil {
		return fmt.Errorf("failed to update outbox task status for id %s: %w", id, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return repository.ErrObjectNotFound
	}
	return nil
}

func (r *OutboxTaskRepo) UpdateTaskStatusTx(ctx context.Context, tx db.Tx, id uuid.UUID, status repository.TaskStatus, attempts int, lastError *string, completedAt *time.Time) error {
	return r.updateTaskStatusInternal(ctx, tx, id, status, attempts, lastError, completedAt)
}

func (r *OutboxTaskRepo) UpdateTaskStatus(ctx context.Context, db db.DB, id uuid.UUID, status repository.TaskStatus, attempts int, lastError *string, completedAt *time.Time) error {
	return r.updateTaskStatusInternal(ctx, db, id, status, attempts, lastError, completedAt)
}
