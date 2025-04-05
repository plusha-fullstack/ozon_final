//go:generate mockgen -source ./filestorage.go -destination=./mocks/filestorage.go -package=mock_storage
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/cache"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
)

type Storage struct {
	db             db.DB
	orderRepo      OrderRepository
	returnRepo     ReturnRepository
	historyRepo    HistoryRepository
	userRepo       UserRepository
	outboxTaskRepo OutboxTaskRepository
	orderCache     *cache.OrderCache
	timeNow        func() time.Time
}

type OrderRepository interface {
	Create(ctx context.Context, order *repository.Order) error
	CreateTx(ctx context.Context, tx db.Tx, order *repository.Order) error
	GetByID(ctx context.Context, id string) (*repository.Order, error)
	Update(ctx context.Context, order *repository.Order) error
	UpdateTx(ctx context.Context, tx db.Tx, order *repository.Order) error
	Delete(ctx context.Context, id string) error
	DeleteTx(ctx context.Context, tx db.Tx, id string) error
	GetByUserID(ctx context.Context, userID string, limit int, activeOnly bool) ([]*repository.Order, error)
	GetByIDTx(ctx context.Context, tx db.Tx, id string) (*repository.Order, error)
	GetAllActiveOrders(ctx context.Context) ([]*repository.Order, error)
}

type ReturnRepository interface {
	Create(ctx context.Context, ret *repository.ReturnEntry) error
	CreateTx(ctx context.Context, tx db.Tx, ret *repository.ReturnEntry) error
	GetPaginated(ctx context.Context, page, limit int) ([]*repository.ReturnEntry, error)
}

type HistoryRepository interface {
	Create(ctx context.Context, entry *repository.HistoryEntry) error
	CreateTx(ctx context.Context, tx db.Tx, entry *repository.HistoryEntry) error
	GetByOrderID(ctx context.Context, orderID string) ([]*repository.HistoryEntry, error)
}
type UserRepository interface {
	CreateUser(ctx context.Context, username, password string) error
	CreateUserTx(ctx context.Context, tx db.Tx, username, password string) error
	ValidateUser(ctx context.Context, username, password string) (bool, error)
}

func NewStorage(
	ctx context.Context,
	db db.DB,
	orderRepo OrderRepository,
	returnRepo ReturnRepository,
	historyRepo HistoryRepository,
	userRepo UserRepository,
	outboxTaskRepo OutboxTaskRepository,
	orderCache *cache.OrderCache,
) *Storage {
	return &Storage{
		db:             db,
		orderRepo:      orderRepo,
		returnRepo:     returnRepo,
		historyRepo:    historyRepo,
		userRepo:       userRepo,
		outboxTaskRepo: outboxTaskRepo,
		orderCache:     orderCache,
		timeNow:        time.Now,
	}
}

func createAuditPayload(action, entityType, entityID string, details map[string]interface{}) json.RawMessage {
	payload := repository.AuditLogPayload{
		Timestamp:  time.Now().UTC(),
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
	}

	if details != nil {
		if status, ok := details["status"].(string); ok {
			payload.NewStatus = status
		}
		if oldStatus, ok := details["old_status"].(string); ok {
			payload.OldStatus = oldStatus
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Failed to marshal audit payload for action %s, entity %s: %v", action, entityID, err)
		return json.RawMessage("{}")
	}
	return json.RawMessage(payloadBytes)
}

func (s *Storage) AddOrder(ctx context.Context, order Order) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var txErr error
	defer func() {
		if txErr != nil {
			log.Printf("Rolling back transaction for AddOrder due to: %v", txErr)
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction: %w", cErr)
				log.Printf("Commit failed for AddOrder: %v", txErr)
			} else {
				repoOrder := &repository.Order{
					ID:           order.ID,
					RecipientID:  order.RecipientID,
					StorageUntil: order.StorageUntil,
					Status:       order.Status,
					Price:        order.Price,
					Weight:       order.Weight,
					Wrapper:      string(order.Wrapper),
					CreatedAt:    order.CreatedAt,
					UpdatedAt:    order.UpdatedAt,
				}
				s.orderCache.Set(repoOrder)
				log.Printf("AddOrder committed, cache updated for %s", order.ID)
			}
		}
	}()

	repoOrder := &repository.Order{
		ID:           order.ID,
		RecipientID:  order.RecipientID,
		StorageUntil: order.StorageUntil,
		Status:       order.Status,
		Price:        order.Price,
		Weight:       order.Weight,
		Wrapper:      string(order.Wrapper),
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}

	txErr = s.orderRepo.CreateTx(ctx, tx, repoOrder)
	if txErr != nil {
		txErr = fmt.Errorf("failed to add order: %w", txErr)
		return txErr
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   order.ID,
		Status:    order.Status,
		ChangedAt: s.timeNow().UTC(),
	}

	txErr = s.historyRepo.CreateTx(ctx, tx, historyEntry)
	if txErr != nil {
		txErr = fmt.Errorf("failed to add order history entry: %w", txErr)
		return txErr
	}

	auditPayload := createAuditPayload("ORDER_CREATED", "order", order.ID, map[string]interface{}{"status": order.Status})
	outboxTask := &repository.OutboxTask{
		Topic:   "audit_logs",
		Payload: auditPayload,
	}
	txErr = s.outboxTaskRepo.CreateTx(ctx, tx, outboxTask)
	if txErr != nil {
		txErr = fmt.Errorf("failed to create outbox task for order creation: %w", txErr)
	}
	return txErr
}

func (s *Storage) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	if cachedOrder, found := s.orderCache.Get(orderID); found {
		log.Printf("Cache HIT for order %s\n", orderID)
		return &Order{
			ID:           cachedOrder.ID,
			RecipientID:  cachedOrder.RecipientID,
			StorageUntil: cachedOrder.StorageUntil,
			Status:       cachedOrder.Status,
			Price:        cachedOrder.Price,
			Weight:       cachedOrder.Weight,
			Wrapper:      Container(cachedOrder.Wrapper),
			CreatedAt:    cachedOrder.CreatedAt,
			UpdatedAt:    cachedOrder.UpdatedAt,
		}, nil
	}
	log.Printf("Cache MISS for order %s\n", orderID)

	repoOrder, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrObjectNotFound) {
			return nil, fmt.Errorf("order not found")
		}
		return nil, fmt.Errorf("failed to get order from DB: %w", err)
	}

	s.orderCache.Set(repoOrder)
	order := &Order{
		ID:           repoOrder.ID,
		RecipientID:  repoOrder.RecipientID,
		StorageUntil: repoOrder.StorageUntil,
		Status:       repoOrder.Status,
		Price:        repoOrder.Price,
		Weight:       repoOrder.Weight,
		Wrapper:      Container(repoOrder.Wrapper),
		CreatedAt:    repoOrder.CreatedAt,
		UpdatedAt:    repoOrder.UpdatedAt,
	}

	return order, nil
}
func (s *Storage) UpdateOrderStatus(ctx context.Context, orderID, status string) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var txErr error
	var updatedOrder *repository.Order
	var oldStatus string

	defer func() {
		if txErr != nil {
			log.Printf("Rolling back transaction for UpdateOrderStatus due to: %v", txErr)
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during UpdateOrderStatus: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during UpdateOrderStatus: %w", cErr)
				log.Printf("Commit failed for UpdateOrderStatus: %v", txErr)
			} else {
				if updatedOrder != nil {
					s.orderCache.Set(updatedOrder)
					log.Printf("UpdateOrderStatus committed, cache updated for %s", orderID)
				}
			}
		}
	}()

	repoOrder, txErr := s.orderRepo.GetByIDTx(ctx, tx, orderID)
	if txErr != nil {
		if errors.Is(txErr, repository.ErrObjectNotFound) {
			txErr = fmt.Errorf("order %s not found", orderID)
		} else {
			txErr = fmt.Errorf("failed to get order %s: %w", orderID, txErr)
		}
		return txErr
	}

	oldStatus = repoOrder.Status

	if repoOrder.Status == status {
		updatedOrder = repoOrder
		return nil
	}

	repoOrder.Status = status
	repoOrder.UpdatedAt = s.timeNow().UTC()
	updatedOrder = repoOrder

	txErr = s.orderRepo.UpdateTx(ctx, tx, repoOrder)
	if txErr != nil {
		txErr = fmt.Errorf("failed to update order %s: %w", orderID, txErr)
		return txErr
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   orderID,
		Status:    status,
		ChangedAt: repoOrder.UpdatedAt,
	}

	txErr = s.historyRepo.CreateTx(ctx, tx, historyEntry)
	if txErr != nil {
		txErr = fmt.Errorf("failed to add order history entry for %s: %w", orderID, txErr)
		return txErr
	}

	auditPayload := createAuditPayload("STATUS_UPDATED", "order", orderID, map[string]interface{}{
		"old_status": oldStatus,
		"new_status": status,
	})
	outboxTask := &repository.OutboxTask{
		Topic:   "audit_logs",
		Payload: auditPayload,
	}
	txErr = s.outboxTaskRepo.CreateTx(ctx, tx, outboxTask)
	if txErr != nil {
		txErr = fmt.Errorf("failed to create outbox task for status update: %w", txErr)
	}

	return txErr
}

func (s *Storage) DeleteOrder(ctx context.Context, orderID string) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var txErr error
	defer func() {
		if txErr != nil {
			log.Printf("Rolling back transaction for DeleteOrder due to: %v", txErr)
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction: %w", cErr)
				log.Printf("Commit failed for DeleteOrder: %v", txErr)
			} else {
				s.orderCache.Delete(orderID)
				log.Printf("DeleteOrder committed, cache updated for %s", orderID)
			}
		}
	}()

	txErr = s.orderRepo.DeleteTx(ctx, tx, orderID)
	if txErr != nil {
		txErr = fmt.Errorf("failed to delete order: %w", txErr)
		return txErr
	}

	auditPayload := createAuditPayload("ORDER_DELETED", "order", orderID, nil)
	outboxTask := &repository.OutboxTask{
		Topic:   "audit_logs",
		Payload: auditPayload,
	}
	txErr = s.outboxTaskRepo.CreateTx(ctx, tx, outboxTask)
	if txErr != nil {
		txErr = fmt.Errorf("failed to create outbox task for order deletion: %w", txErr)
	}
	return txErr
}

func (s *Storage) GetUserOrders(ctx context.Context, userID string, lastN int, activeOnly bool) ([]Order, error) {
	repoOrders, err := s.orderRepo.GetByUserID(ctx, userID, lastN, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to get user orders: %w", err)
	}

	orders := make([]Order, len(repoOrders))
	for i, repoOrder := range repoOrders {
		orders[i] = Order{
			ID:           repoOrder.ID,
			RecipientID:  repoOrder.RecipientID,
			StorageUntil: repoOrder.StorageUntil,
			Status:       repoOrder.Status,
			Price:        repoOrder.Price,
			Weight:       repoOrder.Weight,
			Wrapper:      Container(repoOrder.Wrapper),
			CreatedAt:    repoOrder.CreatedAt,
			UpdatedAt:    repoOrder.UpdatedAt,
		}
	}

	return orders, nil
}

func (s *Storage) AddReturn(ctx context.Context, ret Return) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var txErr error

	defer func() {
		if txErr != nil {
			log.Printf("Rolling back transaction for AddReturn due to: %v", txErr)
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during AddReturn: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during AddReturn: %w", cErr)
				log.Printf("Commit failed for AddReturn: %v", txErr)
			} else {
				s.orderCache.Delete(ret.OrderID)
				log.Printf("AddReturn committed, cache updated for %s", ret.OrderID)
			}
		}
	}()

	repoOrder, txErr := s.orderRepo.GetByIDTx(ctx, tx, ret.OrderID)
	if txErr != nil {
		if errors.Is(txErr, repository.ErrObjectNotFound) {
			txErr = fmt.Errorf("order %s not found", ret.OrderID)
		} else {
			txErr = fmt.Errorf("failed to get order %s for return: %w", ret.OrderID, txErr)
		}
		return txErr
	}

	if repoOrder.RecipientID != ret.UserID {
		txErr = fmt.Errorf("order %s does not belong to user %s", ret.OrderID, ret.UserID)
		return txErr
	}
	if repoOrder.Status != "issued" {
		txErr = fmt.Errorf("order %s not in 'issued' status (current: %s)", ret.OrderID, repoOrder.Status)
		return txErr
	}
	if s.timeNow().UTC().Sub(repoOrder.UpdatedAt) > 48*time.Hour {
		txErr = fmt.Errorf("return period expired for order %s", ret.OrderID)
		return txErr
	}

	oldStatus := repoOrder.Status
	repoOrder.Status = "returned"
	repoOrder.UpdatedAt = s.timeNow().UTC()

	txErr = s.orderRepo.UpdateTx(ctx, tx, repoOrder)
	if txErr != nil {
		txErr = fmt.Errorf("failed to update order %s status to returned: %w", ret.OrderID, txErr)
		return txErr
	}

	repoReturn := &repository.ReturnEntry{
		OrderID:    ret.OrderID,
		UserID:     ret.UserID,
		ReturnedAt: s.timeNow().UTC(),
	}
	txErr = s.returnRepo.CreateTx(ctx, tx, repoReturn)
	if txErr != nil {
		txErr = fmt.Errorf("failed to create return entry for order %s: %w", ret.OrderID, txErr)
		return txErr
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   ret.OrderID,
		Status:    "returned",
		ChangedAt: repoOrder.UpdatedAt,
	}
	txErr = s.historyRepo.CreateTx(ctx, tx, historyEntry)
	if txErr != nil {
		txErr = fmt.Errorf("failed to add order history entry for return %s: %w", ret.OrderID, txErr)
		return txErr
	}

	auditPayload := createAuditPayload("ORDER_RETURNED", "order", ret.OrderID, map[string]interface{}{
		"user_id":    ret.UserID,
		"old_status": oldStatus,
		"new_status": "returned",
	})
	outboxTask := &repository.OutboxTask{
		Topic:   "audit_logs",
		Payload: auditPayload,
	}
	txErr = s.outboxTaskRepo.CreateTx(ctx, tx, outboxTask)
	if txErr != nil {
		txErr = fmt.Errorf("failed to create outbox task for order return: %w", txErr)
	}
	return txErr
}

func (s *Storage) GetReturns(ctx context.Context, page, limit int) ([]Return, error) {
	repoReturns, err := s.returnRepo.GetPaginated(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get returns: %w", err)
	}

	returns := make([]Return, len(repoReturns))
	for i, repoReturn := range repoReturns {
		returns[i] = Return{
			OrderID:    repoReturn.OrderID,
			UserID:     repoReturn.UserID,
			ReturnedAt: repoReturn.ReturnedAt,
		}
	}

	return returns, nil
}

func (s *Storage) GetOrderHistory(ctx context.Context, orderID string) ([]HistoryEntry, error) {
	repoEntries, err := s.historyRepo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order history: %w", err)
	}

	entries := make([]HistoryEntry, len(repoEntries))
	for i, repoEntry := range repoEntries {
		entries[i] = HistoryEntry{
			Status:    repoEntry.Status,
			ChangedAt: repoEntry.ChangedAt,
		}
	}

	return entries, nil
}

type IssueOrdersResult struct {
	OrderID string
	Success bool
	Error   string
}

func (s *Storage) IssueOrders(ctx context.Context, userID string, orderIDs []string) ([]IssueOrdersResult, error) {
	results := make([]IssueOrdersResult, 0, len(orderIDs))
	updatedOrdersForCache := make(map[string]*repository.Order)
	auditTasks := make([]*repository.OutboxTask, 0, len(orderIDs))

	if len(orderIDs) == 0 {
		return results, nil
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for issuing orders: %w", err)
	}
	var txErr error
	defer func() {
		if txErr != nil {
			log.Printf("Rolling back transaction for IssueOrders due to: %v", txErr)
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during IssueOrders: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during IssueOrders: %w", cErr)
				log.Printf("Commit failed for IssueOrders: %v", txErr)
				for i := range results {
					if results[i].Success {
						results[i].Success = false
						results[i].Error = "Transaction commit failed"
					}
				}
			} else {
				log.Printf("IssueOrders committed successfully. Updating cache for %d orders.\n", len(updatedOrdersForCache))
				for _, order := range updatedOrdersForCache {
					s.orderCache.Set(order)
				}
			}
		}
	}()

	issuedStatus := "issued"
	now := s.timeNow().UTC()

	for _, orderID := range orderIDs {
		result := IssueOrdersResult{OrderID: orderID}
		var oldStatus string

		processOrder := func() error {
			repoOrder, err := s.orderRepo.GetByIDTx(ctx, tx, orderID)
			if err != nil {
				if errors.Is(err, repository.ErrObjectNotFound) {
					result.Error = "Order not found"
					return nil
				}
				result.Error = "Failed to retrieve order"
				return fmt.Errorf("failed getting order %s in IssueOrders: %w", orderID, err)
			}
			oldStatus = repoOrder.Status

			if repoOrder.RecipientID != userID {
				result.Error = "Order does not belong to user"
			} else if repoOrder.Status != "received" {
				result.Error = fmt.Sprintf("Order not in 'received' status (current: %s)", repoOrder.Status)
			} else if now.After(repoOrder.StorageUntil) {
				result.Error = "Order storage period has expired"
			}
			if result.Error != "" {
				return nil
			}

			repoOrder.Status = issuedStatus
			repoOrder.UpdatedAt = now
			err = s.orderRepo.UpdateTx(ctx, tx, repoOrder)
			if err != nil {
				result.Error = "Failed to update order status"
				return fmt.Errorf("failed updating order %s in IssueOrders: %w", orderID, err)
			}

			historyEntry := &repository.HistoryEntry{OrderID: orderID, Status: issuedStatus, ChangedAt: now}
			err = s.historyRepo.CreateTx(ctx, tx, historyEntry)
			if err != nil {
				result.Error = "Failed to record history"
				return fmt.Errorf("failed creating history for order %s in IssueOrders: %w", orderID, err)
			}

			result.Success = true
			updatedOrdersForCache[orderID] = repoOrder

			auditPayload := createAuditPayload("ORDER_ISSUED", "order", orderID, map[string]interface{}{
				"user_id":    userID,
				"old_status": oldStatus,
				"new_status": issuedStatus,
			})
			auditTasks = append(auditTasks, &repository.OutboxTask{
				Topic:   "audit_logs",
				Payload: auditPayload,
			})

			return nil
		}

		err = processOrder()
		results = append(results, result)

		if err != nil {
			txErr = err
			break
		}
	}
	if txErr == nil {
		for _, task := range auditTasks {
			err = s.outboxTaskRepo.CreateTx(ctx, tx, task)
			if err != nil {
				txErr = fmt.Errorf("failed to create outbox task during bulk issue: %w", err)
				for i := range results {
					if results[i].Success {
						results[i].Success = false
						results[i].Error = "Failed to create audit log entry"
					}
				}
				break
			}
		}
	}
	return results, txErr
}

type AcceptReturnsResult struct {
	OrderID string
	Success bool
	Error   string
}

func (s *Storage) AcceptReturns(ctx context.Context, userID string, orderIDs []string) ([]AcceptReturnsResult, error) {
	results := make([]AcceptReturnsResult, 0, len(orderIDs))
	deletedOrderIDsFromCache := make(map[string]struct{})
	auditTasks := make([]*repository.OutboxTask, 0, len(orderIDs))

	if len(orderIDs) == 0 {
		return results, nil
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for accepting returns: %w", err)
	}
	var txErr error
	defer func() {
		if txErr != nil {
			log.Printf("Rolling back transaction for AcceptReturns due to: %v", txErr)
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during AcceptReturns: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during AcceptReturns: %w", cErr)
				log.Printf("Commit failed for AcceptReturns: %v", txErr)
				for i := range results {
					if results[i].Success {
						results[i].Success = false
						results[i].Error = "Transaction commit failed"
					}
				}
			} else {
				log.Printf("AcceptReturns committed successfully. Deleting %d orders from cache.\n", len(deletedOrderIDsFromCache))
				for orderID := range deletedOrderIDsFromCache {
					s.orderCache.Delete(orderID)
				}
			}
		}
	}()

	returnedStatus := "returned"
	now := s.timeNow().UTC()

	for _, orderID := range orderIDs {
		result := AcceptReturnsResult{OrderID: orderID}
		var oldStatus string

		processReturn := func() error {
			repoOrder, err := s.orderRepo.GetByIDTx(ctx, tx, orderID)
			if err != nil {
				if errors.Is(err, repository.ErrObjectNotFound) {
					result.Error = "Order not found"
					return nil
				}
				result.Error = "Failed to retrieve order"
				return fmt.Errorf("failed getting order %s in AcceptReturns: %w", orderID, err)
			}
			oldStatus = repoOrder.Status

			if repoOrder.RecipientID != userID {
				result.Error = "Order does not belong to user"
			} else if repoOrder.Status != "issued" {
				result.Error = fmt.Sprintf("Order not in 'issued' status (current: %s)", repoOrder.Status)
			} else if now.Sub(repoOrder.UpdatedAt) > 48*time.Hour {
				result.Error = "Return period expired"
			}
			if result.Error != "" {
				return nil
			}

			repoReturn := &repository.ReturnEntry{OrderID: orderID, UserID: userID, ReturnedAt: now}
			err = s.returnRepo.CreateTx(ctx, tx, repoReturn)
			if err != nil {
				result.Error = "Failed to create return record"
				return fmt.Errorf("failed creating return entry for order %s in AcceptReturns: %w", orderID, err)
			}

			repoOrder.Status = returnedStatus
			repoOrder.UpdatedAt = now
			err = s.orderRepo.UpdateTx(ctx, tx, repoOrder)
			if err != nil {
				result.Error = "Failed to update order status"
				return fmt.Errorf("failed updating order %s in AcceptReturns: %w", orderID, err)
			}

			historyEntry := &repository.HistoryEntry{OrderID: orderID, Status: returnedStatus, ChangedAt: now}
			err = s.historyRepo.CreateTx(ctx, tx, historyEntry)
			if err != nil {
				result.Error = "Failed to record history"
				return fmt.Errorf("failed creating history for order %s in AcceptReturns: %w", orderID, err)
			}

			result.Success = true
			deletedOrderIDsFromCache[orderID] = struct{}{}

			auditPayload := createAuditPayload("ORDER_RETURN_ACCEPTED", "order", orderID, map[string]interface{}{
				"user_id":    userID,
				"old_status": oldStatus,
				"new_status": returnedStatus,
			})
			auditTasks = append(auditTasks, &repository.OutboxTask{
				Topic:   "audit_logs",
				Payload: auditPayload,
			})
			return nil
		}
		err = processReturn()
		results = append(results, result)

		if err != nil {
			txErr = err
			break
		}
	}
	if txErr == nil {
		for _, task := range auditTasks {
			err = s.outboxTaskRepo.CreateTx(ctx, tx, task)
			if err != nil {
				txErr = fmt.Errorf("failed to create outbox task during bulk return acceptance: %w", err)
				for i := range results {
					if results[i].Success {
						results[i].Success = false
						results[i].Error = "Failed to create audit log entry"
					}
				}
				break
			}
		}
	}

	return results, txErr
}
