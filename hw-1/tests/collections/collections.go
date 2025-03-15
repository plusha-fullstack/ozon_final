package collections

import (
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

var (
	Order1 = storage.Order{
		ID:           "af23b8c1-5d13-46c9-8f89-146abcde6721",
		RecipientID:  "user_72a94f3c-5b2e-4e84-a8d1-9c72b0e13f45",
		StorageUntil: time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC),
		Status:       "pending",
		Price:        2499,
		Weight:       12.75,
		Wrapper:      "box",
		CreatedAt:    time.Date(2025, 3, 10, 8, 45, 22, 0, time.UTC),
		UpdatedAt:    time.Date(2025, 3, 10, 8, 45, 22, 0, time.UTC),
	}
	Order2 = storage.Order{
		ID:           "bd45c2e7-8f31-4c69-a124-567def890123",
		RecipientID:  "user_98d72e1f-4a3b-5c7d-8e9f-021abc345d67",
		StorageUntil: time.Date(2025, 5, 23, 0, 0, 0, 0, time.UTC),
		Status:       "processing",
		Price:        3599,
		Weight:       5.30,
		Wrapper:      "envelope",
		CreatedAt:    time.Date(2025, 3, 12, 14, 27, 36, 0, time.UTC),
		UpdatedAt:    time.Date(2025, 3, 13, 9, 15, 42, 0, time.UTC),
	}

	Order3 = storage.Order{
		ID:           "c7e9d3f2-1a5b-4c8d-9e7f-345abc678901",
		RecipientID:  "user_45f8e2d1-9c7b-6a5d-4e3f-210fed789c56",
		StorageUntil: time.Date(2025, 4, 18, 0, 0, 0, 0, time.UTC),
		Status:       "shipped",
		Price:        1275,
		Weight:       8.95,
		Wrapper:      "tube",
		CreatedAt:    time.Date(2025, 3, 5, 11, 32, 14, 0, time.UTC),
		UpdatedAt:    time.Date(2025, 3, 8, 16, 45, 29, 0, time.UTC),
	}
)
