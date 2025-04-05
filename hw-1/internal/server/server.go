//go:generate mockgen -source ./server.go -destination=./mocks/server.go -package=server_repository
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	IssueOrders(ctx context.Context, userID string, orderIDs []string) ([]storage.IssueOrdersResult, error)
	AcceptReturns(ctx context.Context, userID string, orderIDs []string) ([]storage.AcceptReturnsResult, error)
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

func (s *Server) Run(ctx context.Context, port string) error {
	router := s.setupRoutes()

	s.server = &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.AuditManager.Start(ctx)

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

	s.AuditManager.Shutdown(ctx)
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

	if returnRequest.OrderID == "" || returnRequest.UserID == "" {
		respondError(w, http.StatusBadRequest, "Missing order_id or user_id")
		return
	}

	ret := storage.Return{
		OrderID:    returnRequest.OrderID,
		UserID:     returnRequest.UserID,
		ReturnedAt: time.Now().UTC(),
	}

	if err := s.storage.AddReturn(r.Context(), ret); err != nil {

		log.Printf("Error adding return for order %s: %v", returnRequest.OrderID, err)
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not belong") ||
			strings.Contains(err.Error(), "not in 'issued' status") ||
			strings.Contains(err.Error(), "return period expired") {
			respondError(w, http.StatusBadRequest, "Error: "+err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, "Error: failed to process return")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Return accepted and order status updated for order " + returnRequest.OrderID,
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
	if issueRequest.UserID == "" || len(issueRequest.OrderIDs) == 0 {
		respondError(w, http.StatusBadRequest, "Missing user_id or order_ids")
		return
	}

	results, err := s.storage.IssueOrders(r.Context(), issueRequest.UserID, issueRequest.OrderIDs)

	responseMap := make(map[string]string)
	for _, res := range results {
		if res.Success {
			responseMap[res.OrderID] = "Issued successfully"
		} else {
			responseMap[res.OrderID] = fmt.Sprintf("Failed: %s", res.Error)
		}
	}

	if err != nil {
		log.Printf("Transaction error during IssueOrders for user %s: %v", issueRequest.UserID, err)
		respondError(w, http.StatusInternalServerError, "Error processing bulk issue: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, responseMap)
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
	if returnRequest.UserID == "" || len(returnRequest.OrderIDs) == 0 {
		respondError(w, http.StatusBadRequest, "Missing user_id or order_ids")
		return
	}

	results, err := s.storage.AcceptReturns(r.Context(), returnRequest.UserID, returnRequest.OrderIDs)

	responseMap := make(map[string]string)
	for _, res := range results {
		if res.Success {
			responseMap[res.OrderID] = "Return accepted"
		} else {
			responseMap[res.OrderID] = fmt.Sprintf("Failed: %s", res.Error)
		}
	}

	if err != nil {
		log.Printf("Transaction error during AcceptReturns for user %s: %v", returnRequest.UserID, err)
		respondError(w, http.StatusInternalServerError, "Error processing bulk return acceptance: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, responseMap)
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
