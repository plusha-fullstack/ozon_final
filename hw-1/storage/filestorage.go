package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/types"
)

var (
	ErrOrderExists   = errors.New("order already exists")
	ErrOrderNotFound = errors.New("order not found")
	ErrInvalidStatus = errors.New("invalid status transition")
)

type FileStorage struct {
	filePath string
	mu       sync.Mutex
	data     *StorageData
}

type StorageData struct {
	Orders  []types.Order        `json:"orders"`
	Returns []types.Return       `json:"returns"`
	History []types.HistoryEntry `json:"history"`
}

func NewFileStorage(filePath string) (*FileStorage, error) {
	fs := &FileStorage{
		filePath: filePath,
		data:     &StorageData{},
	}
	return fs, fs.load()
}

func (fs *FileStorage) load() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	file, err := os.Open(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(fs.data)
}

func (fs *FileStorage) save() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	file, err := os.Create(fs.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(fs.data)
}

func (fs *FileStorage) AddOrder(order types.Order) error {
	if err := fs.load(); err != nil {
		return err
	}

	for _, o := range fs.data.Orders {
		if o.ID == order.ID {
			return ErrOrderExists
		}
	}

	fs.data.Orders = append(fs.data.Orders, order)
	fs.addHistory(order.ID, order.Status)
	return fs.save()
}

func (fs *FileStorage) addHistory(orderID, status string) {
	entry := types.HistoryEntry{
		OrderID:   orderID,
		Status:    status,
		ChangedAt: time.Now(),
	}
	fs.data.History = append(fs.data.History, entry)
}

func (fs *FileStorage) GetOrder(orderID string) (*types.Order, error) {
	if err := fs.load(); err != nil {
		return nil, err
	}

	for _, o := range fs.data.Orders {
		if o.ID == orderID {
			return &o, nil
		}
	}
	return nil, ErrOrderNotFound
}

func (fs *FileStorage) UpdateOrderStatus(orderID, status string) error {
	if err := fs.load(); err != nil {
		return err
	}

	for i := range fs.data.Orders {
		if fs.data.Orders[i].ID == orderID {
			fs.data.Orders[i].Status = status
			fs.data.Orders[i].UpdatedAt = time.Now()
			fs.addHistory(orderID, status)
			return fs.save()
		}
	}
	return ErrOrderNotFound
}

func (fs *FileStorage) DeleteOrder(orderID string) error {
	if err := fs.load(); err != nil {
		return err
	}

	for i, o := range fs.data.Orders {
		if o.ID == orderID {
			fs.data.Orders = append(fs.data.Orders[:i], fs.data.Orders[i+1:]...)
			return fs.save()
		}
	}
	return ErrOrderNotFound
}

func (fs *FileStorage) AddReturn(ret types.Return) error {
	if err := fs.load(); err != nil {
		return err
	}

	fs.data.Returns = append(fs.data.Returns, ret)
	return fs.save()
}

func (fs *FileStorage) GetReturns(page, limit int) ([]types.Return, error) {
	if err := fs.load(); err != nil {
		return nil, err
	}

	start := (page - 1) * limit
	if start > len(fs.data.Returns) {
		return []types.Return{}, nil
	}
	end := start + limit
	if end > len(fs.data.Returns) {
		end = len(fs.data.Returns)
	}
	return fs.data.Returns[start:end], nil
}

func (fs *FileStorage) GetOrderHistory(orderID string) ([]types.HistoryEntry, error) {
	if err := fs.load(); err != nil {
		return nil, err
	}

	var history []types.HistoryEntry
	for _, h := range fs.data.History {
		if h.OrderID == orderID {
			history = append(history, h)
		}
	}
	return history, nil
}

func (fs *FileStorage) GetUserOrders(userID string, lastN int, activeOnly bool) ([]types.Order, error) {
	if err := fs.load(); err != nil {
		return nil, err
	}

	var orders []types.Order
	for _, o := range fs.data.Orders {
		if o.RecipientID == userID {
			if !activeOnly || o.StorageUntil.After(time.Now()) {
				orders = append(orders, o)
			}
		}
	}

	if lastN > 0 && len(orders) > lastN {
		orders = orders[len(orders)-lastN:]
	}
	return orders, nil
}
