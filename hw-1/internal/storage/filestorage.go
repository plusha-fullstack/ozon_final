//go:generate mockgen -source ./filestorage.go -destination=./mocks/filestorage.go -package=mock_storage
package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/cache"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
)

type Storage struct {
	db          db.DB
	orderRepo   OrderRepository
	returnRepo  ReturnRepository
	historyRepo HistoryRepository
	userRepo    UserRepository
	orderCache  *cache.OrderCache
	timeNow     func() time.Time
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
	orderCache *cache.OrderCache,
) *Storage {
	return &Storage{
		db:          db,
		orderRepo:   orderRepo,
		returnRepo:  returnRepo,
		historyRepo: historyRepo,
		userRepo:    userRepo,
		orderCache:  orderCache,
		timeNow:     time.Now,
	}
}

func (s *Storage) AddOrder(ctx context.Context, order Order) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var txErr error
	defer func() {
		if txErr != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction: %w", cErr)
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

	defer func() {
		if txErr != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during UpdateOrderStatus: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during UpdateOrderStatus: %w", cErr)
			} else {
				if updatedOrder != nil {
					s.orderCache.Set(updatedOrder)
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
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction: %w", cErr)
			} else {
				s.orderCache.Delete(orderID)
			}
		}
	}()

	txErr = s.orderRepo.DeleteTx(ctx, tx, orderID)
	if txErr != nil {
		txErr = fmt.Errorf("failed to delete order: %w", txErr)
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
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during AddReturn: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during AddReturn: %w", cErr)
			} else {
				s.orderCache.Delete(ret.OrderID)
			}
		}
	}()

	repoOrder, txErr := s.orderRepo.GetByIDTx(ctx, tx, ret.OrderID)
	if txErr != nil {
		return txErr
	}
	if txErr != nil {
		return txErr
	}
	if txErr != nil {
		return txErr
	}

	txErr = s.orderRepo.UpdateTx(ctx, tx, repoOrder)
	if txErr != nil {
		return txErr
	}

	if txErr != nil {
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
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during IssueOrders: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during IssueOrders: %w", cErr)
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

			historyEntry := &repository.HistoryEntry{
				OrderID:   orderID,
				Status:    issuedStatus,
				ChangedAt: now,
			}
			err = s.historyRepo.CreateTx(ctx, tx, historyEntry)
			if err != nil {
				result.Error = "Failed to record history"
				return fmt.Errorf("failed creating history for order %s in IssueOrders: %w", orderID, err)
			}

			result.Success = true
			updatedOrdersForCache[orderID] = repoOrder
			return nil
		}

		txErr = processOrder()
		results = append(results, result)

		if txErr != nil {
			break
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
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("ERROR: failed to rollback transaction during AcceptReturns: %v (original error: %v)\n", rbErr, txErr)
			}
		} else {
			if cErr := tx.Commit(ctx); cErr != nil {
				txErr = fmt.Errorf("failed to commit transaction during AcceptReturns: %w", cErr)
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
			return nil
		}

		txErr = processReturn()
		results = append(results, result)

		if txErr != nil {
			break
		}
	}

	return results, txErr
}
