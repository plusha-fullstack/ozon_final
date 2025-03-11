package postgresql

import (
	"context"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/pkg/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/pkg/repository"
)

type HistoryRepo struct {
	db db.DB
}

func NewHistoryRepo(database db.DB) *HistoryRepo {
	return &HistoryRepo{db: database}
}

func (r *HistoryRepo) Create(ctx context.Context, entry *repository.HistoryEntry) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO order_history (
            order_id, status, changed_at
        ) VALUES ($1, $2, $3)
    `, entry.OrderID, entry.Status, entry.ChangedAt)
	return err
}

func (r *HistoryRepo) GetByOrderID(ctx context.Context, orderID string) ([]*repository.HistoryEntry, error) {
	var entries []*repository.HistoryEntry
	err := r.db.Select(ctx, &entries, `
        SELECT * FROM order_history 
        WHERE order_id = $1 
        ORDER BY changed_at ASC
    `, orderID)
	return entries, err
}
