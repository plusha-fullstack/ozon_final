package repository

import (
	"errors"
	"time"
)

var ErrObjectNotFound = errors.New("not found")

type Order struct {
	ID           string    `db:"id"`
	RecipientID  string    `db:"recipient_id"`
	StorageUntil time.Time `db:"storage_until"`
	Status       string    `db:"status"`
	Price        int       `db:"price"`
	Weight       float32   `db:"weight"`
	Wrapper      string    `db:"wrapper"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type ReturnEntry struct {
	ID         int64     `db:"id"`
	OrderID    string    `db:"order_id"`
	UserID     string    `db:"user_id"`
	ReturnedAt time.Time `db:"returned_at"`
}

type HistoryEntry struct {
	ID        int64     `db:"id"`
	OrderID   string    `db:"order_id"`
	Status    string    `db:"status"`
	ChangedAt time.Time `db:"changed_at"`
}
