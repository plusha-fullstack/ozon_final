package server

import (
	"time"
)

type AuditLogEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Handler    string    `json:"handler"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	UserID     string    `json:"user_id,omitempty"`
	OrderID    string    `json:"order_id,omitempty"`
	OldStatus  string    `json:"old_status,omitempty"`
	NewStatus  string    `json:"new_status,omitempty"`
	Request    string    `json:"request,omitempty"`
	Response   string    `json:"response,omitempty"`
}
