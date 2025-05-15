package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusCreated    TaskStatus = "CREATED"
	TaskStatusProcessing TaskStatus = "PROCESSING"
	TaskStatusFailed     TaskStatus = "FAILED"
	TaskStatusDone       TaskStatus = "DONE"
)

type OutboxTask struct {
	ID          uuid.UUID       `db:"id"`
	Status      TaskStatus      `db:"status"`
	Payload     json.RawMessage `db:"payload"`
	Topic       string          `db:"topic"`
	Attempts    int             `db:"attempts"`
	LastError   *string         `db:"last_error"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
	CompletedAt *time.Time      `db:"completed_at"`
}

type AuditLogPayload struct {
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"user_id,omitempty"`
	OrderID    string    `json:"order_id,omitempty"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Handler    string    `json:"handler"`
	StatusCode int       `json:"status_code"`
	Request    string    `json:"request,omitempty"`
	Response   string    `json:"response,omitempty"`
	OldStatus  string    `json:"old_status,omitempty"`
	NewStatus  string    `json:"new_status,omitempty"`
	Action     string    `json:"action"`
	EntityID   string    `json:"entity_id"`
	EntityType string    `json:"entity_type"`
}
