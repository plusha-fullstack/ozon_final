package storage

import "time"

type Order struct {
	ID           string    `json:"id"`
	RecipientID  string    `json:"recipient_id"`
	StorageUntil time.Time `json:"storage_until"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StorageData struct {
	Orders  []Order        `json:"orders"`
	Returns []Return       `json:"returns"`
	History []HistoryEntry `json:"history"`
}

type Return struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	ReturnedAt time.Time `json:"returned_at"`
}

type HistoryEntry struct {
	OrderID   string    `json:"order_id"`
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}
