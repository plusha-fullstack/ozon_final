package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

type OrderRepo struct {
	db db.DB
}

func NewOrderRepo(db db.DB) storage.OrderRepository {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Create(ctx context.Context, order *repository.Order) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO orders (
            id, recipient_id, storage_until, status, price, weight, wrapper, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, order.ID, order.RecipientID, order.StorageUntil, order.Status, order.Price, order.Weight, order.Wrapper, order.CreatedAt, order.UpdatedAt)
	return err
}

func (r *OrderRepo) CreateTx(ctx context.Context, tx db.Tx, order *repository.Order) error {
	_, err := tx.Exec(ctx, `
        INSERT INTO orders (
            id, recipient_id, storage_until, status, price, weight, wrapper, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, order.ID, order.RecipientID, order.StorageUntil, order.Status, order.Price, order.Weight, order.Wrapper, order.CreatedAt, order.UpdatedAt)
	return err
}

func (r *OrderRepo) GetByID(ctx context.Context, id string) (*repository.Order, error) {
	var order repository.Order
	err := r.db.Get(ctx, &order, "SELECT * FROM orders WHERE id = $1", id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrObjectNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepo) Update(ctx context.Context, order *repository.Order) error {
	_, err := r.db.Exec(ctx, `
        UPDATE orders
        SET 
            recipient_id = $1,
            storage_until = $2,
            status = $3,
            price = $4,
            weight = $5,
            wrapper = $6,
            updated_at = $7
        WHERE id = $8
    `, order.RecipientID, order.StorageUntil, order.Status, order.Price, order.Weight, order.Wrapper, order.UpdatedAt, order.ID)
	return err
}

func (r *OrderRepo) UpdateTx(ctx context.Context, tx db.Tx, order *repository.Order) error {
	_, err := tx.Exec(ctx, `
        UPDATE orders
        SET 
            recipient_id = $1,
            storage_until = $2,
            status = $3,
            price = $4,
            weight = $5,
            wrapper = $6,
            updated_at = $7
        WHERE id = $8
    `, order.RecipientID, order.StorageUntil, order.Status, order.Price, order.Weight, order.Wrapper, order.UpdatedAt, order.ID)
	return err
}

func (r *OrderRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM orders WHERE id = $1", id)
	return err
}

func (r *OrderRepo) DeleteTx(ctx context.Context, tx db.Tx, id string) error {
	_, err := tx.Exec(ctx, "DELETE FROM orders WHERE id = $1", id)
	return err
}

func (r *OrderRepo) GetByUserID(ctx context.Context, userID string, limit int, activeOnly bool) ([]*repository.Order, error) {
	query := "SELECT * FROM orders WHERE recipient_id = $1"
	args := []interface{}{userID}

	if activeOnly {
		query += " AND status NOT IN ('issued', 'returned')"
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}

	var orders []*repository.Order
	err := r.db.Select(ctx, &orders, query, args...)
	return orders, err
}

func (r *OrderRepo) GetByIDTx(ctx context.Context, tx db.Tx, id string) (*repository.Order, error) {
	var order repository.Order
	err := tx.Get(ctx, &order, "SELECT * FROM orders WHERE id = $1 FOR UPDATE", id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrObjectNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepo) GetAllActiveOrders(ctx context.Context) ([]*repository.Order, error) {
	query := `
        SELECT * FROM orders 
        WHERE status = 'received' OR status = 'issued'
        ORDER BY created_at ASC 
    `
	var orders []*repository.Order
	err := r.db.Select(ctx, &orders, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all active orders: %w", err)
	}
	return orders, nil
}
