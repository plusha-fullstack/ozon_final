package cache

import (
	"context"
	"log"
	"sync"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
)

type OrderRepository interface {
	GetAllActiveOrders(ctx context.Context) ([]*repository.Order, error)
}

type OrderCache struct {
	mu    sync.RWMutex
	cache map[string]*repository.Order
	repo  OrderRepository
}

func NewOrderCache(repo OrderRepository) *OrderCache {
	return &OrderCache{
		cache: make(map[string]*repository.Order),
		repo:  repo,
	}
}

func (c *OrderCache) LoadInitialData(ctx context.Context) error {
	log.Println("Loading initial data into order cache...")
	orders, err := c.repo.GetAllActiveOrders(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, order := range orders {
		orderCopy := *order
		c.cache[order.ID] = &orderCopy
	}
	log.Printf("Successfully loaded %d active orders into cache.", len(c.cache))
	return nil
}

func (c *OrderCache) Get(orderID string) (*repository.Order, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	order, found := c.cache[orderID]
	if !found {
		return nil, false
	}
	orderCopy := *order
	return &orderCopy, true
}

func (c *OrderCache) Set(order *repository.Order) {
	if !isActiveStatus(order.Status) {
		c.Delete(order.ID)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	orderCopy := *order
	c.cache[order.ID] = &orderCopy
	log.Printf("Cache: Set order %s (Status: %s)\n", order.ID, order.Status)
}

func (c *OrderCache) Delete(orderID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, found := c.cache[orderID]; found {
		delete(c.cache, orderID)
		log.Printf("Cache: Deleted order %s\n", orderID)
	}
}

func isActiveStatus(status string) bool {
	return status == "received" || status == "issued"
}
