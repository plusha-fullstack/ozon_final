package postgresql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock_database "gitlab.ozon.dev/pupkingeorgij/homework/internal/db/mocks"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"go.uber.org/mock/gomock"
)

func TestHistoryRepo_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewHistoryRepo(mockDB)
		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.HistoryEntry{
			OrderID:   "order123",
			Status:    "delivered",
			ChangedAt: changed.UTC(),
		}

		mockDB.EXPECT().
			Exec(gomock.Any(), gomock.Any(),
				gomock.Eq(entry.OrderID),
				gomock.Eq(entry.Status),
				gomock.Eq(entry.ChangedAt)).
			Return(nil, nil)

		err = repo.Create(ctx, entry)
		assert.NoError(t, err)
	})

	t.Run("DB Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewHistoryRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.HistoryEntry{
			OrderID:   "order123",
			Status:    "delivered",
			ChangedAt: changed.UTC(),
		}
		dbErr := errors.New("database error")

		mockDB.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, dbErr)

		err = repo.Create(ctx, entry)
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
	})
}

func TestHistoryRepo_CreateTx(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		mockTx := mock_database.NewMockTx(ctrl)
		repo := NewHistoryRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.HistoryEntry{
			OrderID:   "order123",
			Status:    "delivered",
			ChangedAt: changed.UTC(),
		}

		mockTx.EXPECT().
			Exec(gomock.Any(), gomock.Any(),
				gomock.Eq(entry.OrderID),
				gomock.Eq(entry.Status),
				gomock.Eq(entry.ChangedAt)).
			Return(nil, nil)

		err = repo.CreateTx(ctx, mockTx, entry)
		assert.NoError(t, err)
	})

	t.Run("Tx Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		mockTx := mock_database.NewMockTx(ctrl)
		repo := NewHistoryRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.HistoryEntry{
			OrderID:   "order123",
			Status:    "delivered",
			ChangedAt: changed.UTC(),
		}
		txErr := errors.New("transaction error")

		mockTx.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, txErr)

		err = repo.CreateTx(ctx, mockTx, entry)
		assert.Error(t, err)
		assert.Equal(t, txErr, err)
	})
}

func TestHistoryRepo_GetByOrderID(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewHistoryRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2026-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		orderID := "order123"
		expectedEntries := []*repository.HistoryEntry{
			{
				ID:        1,
				OrderID:   orderID,
				Status:    "created",
				ChangedAt: changed.UTC().Add(-2 * time.Hour),
			},
			{
				ID:        2,
				OrderID:   orderID,
				Status:    "processing",
				ChangedAt: changed.UTC().Add(-1 * time.Hour),
			},
			{
				ID:        3,
				OrderID:   orderID,
				Status:    "delivered",
				ChangedAt: changed.UTC(),
			},
		}

		entries, err := repo.GetByOrderID(ctx, orderID)
		assert.NoError(t, err)
		assert.Equal(t, expectedEntries, entries)
	})

	t.Run("Empty Result", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewHistoryRepo(mockDB)

		orderID := "nonexistent"

		entries, err := repo.GetByOrderID(ctx, orderID)
		assert.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("DB Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewHistoryRepo(mockDB)

		orderID := "order123"
		dbErr := errors.New("database error")

		mockDB.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(orderID)).
			Return(dbErr)

		entries, err := repo.GetByOrderID(ctx, orderID)
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
		assert.Nil(t, entries)
	})
}
