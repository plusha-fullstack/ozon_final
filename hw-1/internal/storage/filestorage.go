package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"
)

var (
	ErrOrderExists   = errors.New("order already exists")
	ErrOrderNotFound = errors.New("order not found")
)

type FileStorage struct {
	filePath string
	data     *StorageData
}

func New(filePath string) (*FileStorage, error) {
	fs := &FileStorage{
		filePath: filePath,
		data: &StorageData{
			Orders:  make([]Order, 0),
			Returns: make([]Return, 0),
			History: make([]HistoryEntry, 0),
		},
	}
	return fs, fs.load()
}

func (fs *FileStorage) load() error {
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
	file, err := os.Create(fs.filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(fs.data)
}

func (fs *FileStorage) AddOrder(order Order) error {
	for _, o := range fs.data.Orders {
		if o.ID == order.ID {
			return ErrOrderExists
		}
	}

	fs.data.Orders = append(fs.data.Orders, order)
	fs.addHistory(order.ID, "received")
	return fs.save()
}

func (fs *FileStorage) GetOrder(orderID string) (*Order, error) {
	for _, o := range fs.data.Orders {
		if o.ID == orderID {
			return &o, nil
		}
	}
	return nil, ErrOrderNotFound
}

func (fs *FileStorage) UpdateOrderStatus(orderID, status string) error {
	for i := range fs.data.Orders {
		if fs.data.Orders[i].ID == orderID {
			fs.data.Orders[i].Status = status
			fs.data.Orders[i].UpdatedAt = time.Now().UTC()
			fs.addHistory(orderID, status)
			return fs.save()
		}
	}
	return ErrOrderNotFound
}

func (fs *FileStorage) DeleteOrder(orderID string) error {
	for i, o := range fs.data.Orders {
		if o.ID == orderID {
			fs.data.Orders = append(fs.data.Orders[:i], fs.data.Orders[i+1:]...)
			return fs.save()
		}
	}
	return ErrOrderNotFound
}

func (fs *FileStorage) GetUserOrders(userID string, lastN int, activeOnly bool) ([]Order, error) {
	var result []Order
	for _, o := range fs.data.Orders {
		if o.RecipientID == userID {
			if !activeOnly || o.StorageUntil.After(time.Now().UTC()) {
				result = append(result, o)
			}
		}
	}

	if lastN > 0 && len(result) > lastN {
		result = result[len(result)-lastN:]
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (fs *FileStorage) AddReturn(ret Return) error {
	fs.data.Returns = append(fs.data.Returns, ret)
	return fs.save()
}

func (fs *FileStorage) GetReturns(page, limit int) ([]Return, error) {
	start := (page - 1) * limit
	if start >= len(fs.data.Returns) {
		return []Return{}, nil
	}

	end := start + limit
	if end > len(fs.data.Returns) {
		end = len(fs.data.Returns)
	}

	return fs.data.Returns[start:end], nil
}

func (fs *FileStorage) GetOrderHistory(orderID string) ([]HistoryEntry, error) {
	var history []HistoryEntry
	for _, h := range fs.data.History {
		if h.OrderID == orderID {
			history = append(history, h)
		}
	}
	return history, nil
}

func (fs *FileStorage) addHistory(orderID, status string) {
	fs.data.History = append(fs.data.History, HistoryEntry{
		OrderID:   orderID,
		Status:    status,
		ChangedAt: time.Now().UTC(),
	})
}
