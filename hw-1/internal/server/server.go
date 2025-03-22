//go:generate mockgen -source ./server.go -destination=./mocks/server.go -package=server_repository
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

type Storage interface {
	AddOrder(ctx context.Context, order storage.Order) error
	GetOrder(ctx context.Context, orderID string) (*storage.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID, status string) error
	DeleteOrder(ctx context.Context, orderID string) error
	GetUserOrders(ctx context.Context, userID string, lastN int, activeOnly bool) ([]storage.Order, error)
	AddReturn(ctx context.Context, ret storage.Return) error
	GetReturns(ctx context.Context, page, limit int) ([]storage.Return, error)
	GetOrderHistory(ctx context.Context, orderID string) ([]storage.HistoryEntry, error)
}

type UserRepo interface {
	ValidateUser(ctx context.Context, username, password string) (bool, error)
}

type Server struct {
	storage      Storage
	userRepo     UserRepo
	server       *http.Server
	AuditManager *AuditManager
}

func New(storage Storage, userRepo UserRepo) *Server {
	auditManager := NewAuditManager(2, 5, 500*time.Millisecond)
	return &Server{
		storage:      storage,
		userRepo:     userRepo,
		AuditManager: auditManager,
	}
}

func (s *Server) Run(port string) error {
	router := s.setupRoutes()

	s.server = &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.AuditManager.Start()

	go s.handleShutdown()

	log.Printf("Server starting on port %s", port)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleShutdown() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signals
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Printf("ERROR: Server shutdown failed: %v", err)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}
	log.Println("HTTP server shutdown completed")

	s.AuditManager.Shutdown()
	log.Println("Server shutdown completed successfully")

	return nil
}
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	handler := s.auditLogMiddleware(s.basicAuthMiddleware(mux))

	mux.HandleFunc("POST /orders", s.handleCreateOrder)
	mux.HandleFunc("GET /orders/{id}", s.handleGetOrder)
	mux.HandleFunc("PUT /orders/{id}/status", s.handleUpdateOrderStatus)
	mux.HandleFunc("DELETE /orders/{id}", s.handleDeleteOrder)
	mux.HandleFunc("GET /users/{userID}/orders", s.handleListOrders)

	mux.HandleFunc("POST /returns", s.handleAddReturn)
	mux.HandleFunc("GET /returns", s.handleListReturns)

	mux.HandleFunc("POST /process/issue", s.handleIssueOrders)
	mux.HandleFunc("POST /process/return", s.handleAcceptReturns)

	mux.HandleFunc("GET /orders/{id}/history", s.handleOrderHistory)

	return handler
}

func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		valid, err := s.userRepo.ValidateUser(r.Context(), username, password)
		if err != nil || !valid {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var orderRequest struct {
		ID                     string  `json:"id"`
		RecipientID            string  `json:"recipient_id"`
		WrapperType            string  `json:"wrapper_type"`
		Price                  int     `json:"price"`
		Weight                 float32 `json:"weight"`
		StorageUntil           string  `json:"storage_until"`
		WithAdditionalMembrane bool    `json:"with_additional_membrane"`
	}

	if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	date, err := time.Parse("2006-01-02", orderRequest.StorageUntil)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
		return
	}

	if date.Before(time.Now()) {
		respondError(w, http.StatusBadRequest, "Error: storage period is in the past")
		return
	}

	packager, err := storage.GetPackager(orderRequest.WrapperType, orderRequest.WithAdditionalMembrane)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	if err := packager.ValidateWeight(orderRequest.Weight); err != nil {
		respondError(w, http.StatusBadRequest, "Validation Failed: "+err.Error())
		return
	}

	adjustedPrice := packager.AdjustPrice(orderRequest.Price)

	order := storage.Order{
		ID:           orderRequest.ID,
		RecipientID:  orderRequest.RecipientID,
		StorageUntil: date.UTC(),
		Status:       "received",
		Price:        adjustedPrice,
		Weight:       orderRequest.Weight,
		Wrapper:      storage.Container(packager.GetType()),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.storage.AddOrder(r.Context(), order); err != nil {
		respondError(w, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Order accepted successfully",
		"id":      order.ID,
	})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	if orderID == "" {
		respondError(w, http.StatusBadRequest, "Missing order ID")
		return
	}

	order, err := s.storage.GetOrder(r.Context(), orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, order)
}

func (s *Server) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	if orderID == "" {
		respondError(w, http.StatusBadRequest, "Missing order ID")
		return
	}

	var statusRequest struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&statusRequest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.storage.UpdateOrderStatus(r.Context(), orderID, statusRequest.Status); err != nil {
		respondError(w, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Order status updated successfully",
	})
}

func (s *Server) handleDeleteOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	if orderID == "" {
		respondError(w, http.StatusBadRequest, "Missing order ID")
		return
	}

	order, err := s.storage.GetOrder(r.Context(), orderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Error: "+err.Error())
		return
	}

	if order.Status != "received" {
		respondError(w, http.StatusBadRequest, "Cannot return: order not in 'received' status")
		return
	}

	if time.Now().UTC().Before(order.StorageUntil) {
		respondError(w, http.StatusBadRequest, "Cannot return: storage period not expired")
		return
	}

	if err := s.storage.DeleteOrder(r.Context(), orderID); err != nil {
		respondError(w, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Order returned to courier",
	})
}

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "Missing user ID")
		return
	}

	lastN := 0
	activeOnly := false

	if lastNStr := r.URL.Query().Get("last"); lastNStr != "" {
		var err error
		lastN, err = strconv.Atoi(lastNStr)
		if err != nil || lastN <= 0 {
			respondError(w, http.StatusBadRequest, "Invalid value for 'last' parameter")
			return
		}
	}

	if activeStr := r.URL.Query().Get("active"); activeStr == "true" {
		activeOnly = true
	}

	orders, err := s.storage.GetUserOrders(r.Context(), userID, lastN, activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, orders)
}

func (s *Server) handleAddReturn(w http.ResponseWriter, r *http.Request) {
	var returnRequest struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&returnRequest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	order, err := s.storage.GetOrder(r.Context(), returnRequest.OrderID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Error: order not found")
		return
	}

	if order.RecipientID != returnRequest.UserID {
		respondError(w, http.StatusBadRequest, "Error: order does not belong to user")
		return
	}

	if order.Status != "issued" {
		respondError(w, http.StatusBadRequest, "Error: order is not in 'issued' status")
		return
	}

	if time.Since(order.UpdatedAt) > 48*time.Hour {
		respondError(w, http.StatusBadRequest, "Error: return period expired for order")
		return
	}

	ret := storage.Return{
		OrderID:    returnRequest.OrderID,
		UserID:     returnRequest.UserID,
		ReturnedAt: time.Now().UTC(),
	}

	if err := s.storage.AddReturn(r.Context(), ret); err != nil {
		respondError(w, http.StatusInternalServerError, "Error: failed to accept return")
		return
	}

	if err := s.storage.UpdateOrderStatus(r.Context(), returnRequest.OrderID, "returned"); err != nil {
		respondError(w, http.StatusInternalServerError, "Error: failed to update order status")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Return accepted for order",
	})
}

func (s *Server) handleListReturns(w http.ResponseWriter, r *http.Request) {
	page := 1
	limit := 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil || page <= 0 {
			respondError(w, http.StatusBadRequest, "Invalid value for 'page' parameter")
			return
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			respondError(w, http.StatusBadRequest, "Invalid value for 'limit' parameter")
			return
		}
	}

	returns, err := s.storage.GetReturns(r.Context(), page, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, returns)
}

func (s *Server) handleIssueOrders(w http.ResponseWriter, r *http.Request) {
	var issueRequest struct {
		UserID   string   `json:"user_id"`
		OrderIDs []string `json:"order_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&issueRequest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make(map[string]string)

	for _, orderID := range issueRequest.OrderIDs {
		order, err := s.storage.GetOrder(r.Context(), orderID)
		if err != nil {
			results[orderID] = "Order not found"
			continue
		}

		if order.RecipientID != issueRequest.UserID {
			results[orderID] = "Order does not belong to user"
			continue
		}

		if order.Status != "received" {
			results[orderID] = "Order is not in 'received' status"
			continue
		}

		if time.Now().UTC().After(order.StorageUntil) {
			results[orderID] = "Order has expired"
			continue
		}

		if err := s.storage.UpdateOrderStatus(r.Context(), orderID, "issued"); err != nil {
			results[orderID] = "Failed to issue order"
		} else {
			results[orderID] = "Issued successfully"
		}
	}

	respondJSON(w, http.StatusOK, results)
}

func (s *Server) handleAcceptReturns(w http.ResponseWriter, r *http.Request) {
	var returnRequest struct {
		UserID   string   `json:"user_id"`
		OrderIDs []string `json:"order_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&returnRequest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make(map[string]string)

	for _, orderID := range returnRequest.OrderIDs {
		order, err := s.storage.GetOrder(r.Context(), orderID)
		if err != nil {
			results[orderID] = "Order not found"
			continue
		}

		if order.RecipientID != returnRequest.UserID {
			results[orderID] = "Order does not belong to user"
			continue
		}

		if order.Status != "issued" {
			results[orderID] = "Order is not in 'issued' status"
			continue
		}

		if time.Since(order.UpdatedAt) > 48*time.Hour {
			results[orderID] = "Return period expired"
			continue
		}

		ret := storage.Return{
			OrderID:    orderID,
			UserID:     returnRequest.UserID,
			ReturnedAt: time.Now().UTC(),
		}

		if err := s.storage.AddReturn(r.Context(), ret); err != nil {
			results[orderID] = "Failed to accept return"
		} else {
			s.storage.UpdateOrderStatus(r.Context(), orderID, "returned")
			results[orderID] = "Return accepted"
		}
	}

	respondJSON(w, http.StatusOK, results)
}

func (s *Server) handleOrderHistory(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	if orderID == "" {
		respondError(w, http.StatusBadRequest, "Missing order ID")
		return
	}

	history, err := s.storage.GetOrderHistory(r.Context(), orderID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, history)
}
