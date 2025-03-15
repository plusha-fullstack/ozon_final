package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
)

type Storage struct {
	db          db.DB
	orderRepo   OrderRepository
	returnRepo  ReturnRepository
	historyRepo HistoryRepository
	userRepo    UserRepository
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
) *Storage {
	return &Storage{
		db:          db,
		orderRepo:   orderRepo,
		returnRepo:  returnRepo,
		historyRepo: historyRepo,
		userRepo:    userRepo,
		timeNow:     time.Now,
	}
}

func (s *Storage) AddOrder(ctx context.Context, order Order) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v\n", rbErr)
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

	err = s.orderRepo.CreateTx(ctx, tx, repoOrder)
	if err != nil {
		return fmt.Errorf("failed to add order: %w", err)
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   order.ID,
		Status:    order.Status,
		ChangedAt: s.timeNow().UTC(),
	}

	err = s.historyRepo.CreateTx(ctx, tx, historyEntry)
	if err != nil {
		return fmt.Errorf("failed to add order history entry: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	repoOrder, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrObjectNotFound) {
			return nil, fmt.Errorf("order not found")
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

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

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v\n", rbErr)
			}
		}
	}()

	repoOrder, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrObjectNotFound) {
			return fmt.Errorf("order not found")
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	repoOrder.Status = status
	repoOrder.UpdatedAt = time.Now().UTC()

	err = s.orderRepo.UpdateTx(ctx, tx, repoOrder)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   orderID,
		Status:    status,
		ChangedAt: s.timeNow().UTC(),
	}

	err = s.historyRepo.CreateTx(ctx, tx, historyEntry)
	if err != nil {
		return fmt.Errorf("failed to add order history entry: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Storage) DeleteOrder(ctx context.Context, orderID string) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v\n", rbErr)
			}
		}
	}()

	err = s.orderRepo.DeleteTx(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				fmt.Printf("failed to rollback transaction: %v\n", rbErr)
			}
		}
	}()

	repoReturn := &repository.ReturnEntry{
		OrderID:    ret.OrderID,
		UserID:     ret.UserID,
		ReturnedAt: ret.ReturnedAt,
	}

	err = s.returnRepo.CreateTx(ctx, tx, repoReturn)
	if err != nil {
		return fmt.Errorf("failed to add return: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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
