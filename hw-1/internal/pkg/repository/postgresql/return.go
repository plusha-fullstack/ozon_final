package postgresql

import (
	"context"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/pkg/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/pkg/repository"
)

type ReturnRepo struct {
	db db.DB
}

func NewReturnRepo(database db.DB) *ReturnRepo {
	return &ReturnRepo{db: database}
}

func (r *ReturnRepo) Create(ctx context.Context, ret *repository.ReturnEntry) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO returns (
            order_id, user_id, returned_at
        ) VALUES ($1, $2, $3)
    `, ret.OrderID, ret.UserID, ret.ReturnedAt)
	return err
}

func (r *ReturnRepo) GetPaginated(ctx context.Context, page, limit int) ([]*repository.ReturnEntry, error) {
	offset := (page - 1) * limit

	var returns []*repository.ReturnEntry
	err := r.db.Select(ctx, &returns, `
        SELECT * FROM returns 
        ORDER BY returned_at DESC 
        LIMIT $1 OFFSET $2
    `, limit, offset)
	return returns, err
}
