package postgresql

import (
	"context"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

type HistoryRepo struct {
	db db.DB
}

func NewHistoryRepo(db db.DB) storage.HistoryRepository {
	return &HistoryRepo{db: db}
}
func (r *HistoryRepo) Create(ctx context.Context, entry *repository.HistoryEntry) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO order_history (
            order_id, status, changed_at
        ) VALUES ($1, $2, $3)
    `, entry.OrderID, entry.Status, entry.ChangedAt)
	return err
}

func (r *HistoryRepo) CreateTx(ctx context.Context, tx db.Tx, entry *repository.HistoryEntry) error {
	_, err := tx.Exec(ctx, `
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
