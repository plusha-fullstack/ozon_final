package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) auditLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		skipRequestBody := strings.Contains(contentType, "multipart/form-data")
		entry := AuditLogEntry{
			Timestamp: time.Now(),
			Method:    r.Method,
			Path:      r.URL.Path,
			Handler:   getHandlerName(r.URL.Path, r.Method),
		}

		if username, _, ok := r.BasicAuth(); ok {
			entry.UserID = username
		}

		if strings.Contains(r.URL.Path, "/orders/") {
			parts := strings.Split(r.URL.Path, "/")
			for i, part := range parts {
				if part == "orders" && i+1 < len(parts) {
					entry.OrderID = parts[i+1]
					break
				}
			}
		}

		var requestBody []byte
		if !skipRequestBody && r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			entry.Request = string(requestBody)

			if entry.OrderID != "" && strings.Contains(r.URL.Path, "/status") {
				var statusRequest struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal(requestBody, &statusRequest); err == nil {
					if order, err := s.storage.GetOrder(r.Context(), entry.OrderID); err == nil {
						entry.OldStatus = order.Status
						entry.NewStatus = statusRequest.Status
					}
				}
			}
		}

		wrw := newResponseWriterWrapper(w)

		next.ServeHTTP(wrw, r)

		entry.StatusCode = wrw.GetStatusCode()
		entry.Response = string(wrw.GetBody())

		s.AuditManager.LogEntry(r.Context(), entry)
	})
}

func getHandlerName(path string, method string) string {
	if strings.HasPrefix(path, "/orders") {
		if method == "POST" {
			return "handleCreateOrder"
		} else if strings.Contains(path, "/status") {
			return "handleUpdateOrderStatus"
		} else if method == "GET" && strings.Contains(path, "/history") {
			return "handleOrderHistory"
		} else if method == "GET" {
			return "handleGetOrder"
		} else if method == "DELETE" {
			return "handleDeleteOrder"
		}
	} else if strings.HasPrefix(path, "/users") && strings.Contains(path, "/orders") {
		return "handleListOrders"
	} else if strings.HasPrefix(path, "/returns") {
		if method == "POST" {
			return "handleAddReturn"
		} else {
			return "handleListReturns"
		}
	} else if strings.HasPrefix(path, "/process/issue") {
		return "handleIssueOrders"
	} else if strings.HasPrefix(path, "/process/return") {
		return "handleAcceptReturns"
	}

	return "unknown"
}
