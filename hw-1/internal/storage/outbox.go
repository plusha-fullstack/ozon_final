package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
)

type OutboxTaskRepository interface {
	CreateTx(ctx context.Context, tx db.Tx, task *repository.OutboxTask) error
	GetProcessableTasks(ctx context.Context, db db.DB, limit int) ([]*repository.OutboxTask, error)
	UpdateTaskStatusTx(ctx context.Context, tx db.Tx, id uuid.UUID, status repository.TaskStatus, attempts int, lastError *string, completedAt *time.Time) error
	UpdateTaskStatus(ctx context.Context, db db.DB, id uuid.UUID, status repository.TaskStatus, attempts int, lastError *string, completedAt *time.Time) error
}
