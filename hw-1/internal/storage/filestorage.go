package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/pkg/repository"
)

type PostgresStorage struct {
	orderRepo   OrderRepository
	returnRepo  ReturnRepository
	historyRepo HistoryRepository
	ctx         context.Context
}

type OrderRepository interface {
	Create(ctx context.Context, order *repository.Order) error
	GetByID(ctx context.Context, id string) (*repository.Order, error)
	Update(ctx context.Context, order *repository.Order) error
	Delete(ctx context.Context, id string) error
	GetByUserID(ctx context.Context, userID string, limit int, activeOnly bool) ([]*repository.Order, error)
}

type ReturnRepository interface {
	Create(ctx context.Context, ret *repository.ReturnEntry) error
	GetPaginated(ctx context.Context, page, limit int) ([]*repository.ReturnEntry, error)
}

type HistoryRepository interface {
	Create(ctx context.Context, entry *repository.HistoryEntry) error
	GetByOrderID(ctx context.Context, orderID string) ([]*repository.HistoryEntry, error)
}

func NewPostgresStorage(
	ctx context.Context,
	orderRepo OrderRepository,
	returnRepo ReturnRepository,
	historyRepo HistoryRepository,
) *PostgresStorage {
	return &PostgresStorage{
		orderRepo:   orderRepo,
		returnRepo:  returnRepo,
		historyRepo: historyRepo,
		ctx:         ctx,
	}
}

func (s *PostgresStorage) AddOrder(order Order) error {
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

	err := s.orderRepo.Create(s.ctx, repoOrder)
	if err != nil {
		return fmt.Errorf("failed to add order: %w", err)
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   order.ID,
		Status:    order.Status,
		ChangedAt: time.Now().UTC(),
	}

	if err := s.historyRepo.Create(s.ctx, historyEntry); err != nil {
		return fmt.Errorf("failed to add order history entry: %w", err)
	}

	return nil
}

func (s *PostgresStorage) GetOrder(orderID string) (*Order, error) {
	repoOrder, err := s.orderRepo.GetByID(s.ctx, orderID)
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

func (s *PostgresStorage) UpdateOrderStatus(orderID, status string) error {
	repoOrder, err := s.orderRepo.GetByID(s.ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrObjectNotFound) {
			return fmt.Errorf("order not found")
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	repoOrder.Status = status
	repoOrder.UpdatedAt = time.Now().UTC()
	if err := s.orderRepo.Update(s.ctx, repoOrder); err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	historyEntry := &repository.HistoryEntry{
		OrderID:   orderID,
		Status:    status,
		ChangedAt: time.Now().UTC(),
	}

	if err := s.historyRepo.Create(s.ctx, historyEntry); err != nil {
		return fmt.Errorf("failed to add order history entry: %w", err)
	}

	return nil
}

func (s *PostgresStorage) DeleteOrder(orderID string) error {
	if err := s.orderRepo.Delete(s.ctx, orderID); err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}
	return nil
}

func (s *PostgresStorage) GetUserOrders(userID string, lastN int, activeOnly bool) ([]Order, error) {
	repoOrders, err := s.orderRepo.GetByUserID(s.ctx, userID, lastN, activeOnly)
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

func (s *PostgresStorage) AddReturn(ret Return) error {
	repoReturn := &repository.ReturnEntry{
		OrderID:    ret.OrderID,
		UserID:     ret.UserID,
		ReturnedAt: ret.ReturnedAt,
	}

	if err := s.returnRepo.Create(s.ctx, repoReturn); err != nil {
		return fmt.Errorf("failed to add return: %w", err)
	}

	return nil
}

func (s *PostgresStorage) GetReturns(page, limit int) ([]Return, error) {
	repoReturns, err := s.returnRepo.GetPaginated(s.ctx, page, limit)
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

func (s *PostgresStorage) GetOrderHistory(orderID string) ([]HistoryEntry, error) {
	repoEntries, err := s.historyRepo.GetByOrderID(s.ctx, orderID)
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
