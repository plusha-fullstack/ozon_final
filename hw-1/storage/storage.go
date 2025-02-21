package storage

import "gitlab.ozon.dev/pupkingeorgij/homework/types"

type Storage interface {
	AddOrder(order types.Order) error
	GetOrder(orderID string) (*types.Order, error)
	UpdateOrderStatus(orderID, status string) error
	DeleteOrder(orderID string) error
	AddReturn(ret types.Return) error
	GetReturns(page, limit int) ([]types.Return, error)
	GetOrderHistory(orderID string) ([]types.HistoryEntry, error)
	GetUserOrders(userID string, lastN int, activeOnly bool) ([]types.Order, error)
}
