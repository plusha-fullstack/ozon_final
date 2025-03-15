package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	mock_db "gitlab.ozon.dev/pupkingeorgij/homework/internal/db/mocks"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	mock_storage "gitlab.ozon.dev/pupkingeorgij/homework/internal/storage/mocks"
	"go.uber.org/mock/gomock"
)

func TestStorage_AddOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockTx := mock_db.NewMockTx(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)
	storage.timeNow = func() time.Time { return fixedTime }

	t.Run("successful order creation", func(t *testing.T) {
		order := Order{
			ID:           "order-123",
			RecipientID:  "user-456",
			StorageUntil: time.Now().Add(24 * time.Hour),
			Status:       "new",
			Price:        100,
			Weight:       2.5,
			Wrapper:      "box",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		expectedRepoOrder := &repository.Order{
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

		expectedHistoryEntry := &repository.HistoryEntry{
			OrderID:   order.ID,
			Status:    order.Status,
			ChangedAt: fixedTime.UTC(),
		}

		// Set up expectations
		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ db.Tx, o *repository.Order) error {
				assert.Equal(t, expectedRepoOrder.ID, o.ID)
				assert.Equal(t, expectedRepoOrder.RecipientID, o.RecipientID)
				assert.Equal(t, expectedRepoOrder.Status, o.Status)
				return nil
			})
		mockHistoryRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ db.Tx, h *repository.HistoryEntry) error {
				assert.Equal(t, expectedHistoryEntry.OrderID, h.OrderID)
				assert.Equal(t, expectedHistoryEntry.Status, h.Status)
				assert.Equal(t, expectedHistoryEntry.ChangedAt, h.ChangedAt)
				return nil
			})
		mockTx.EXPECT().Commit(ctx).Return(nil)

		// Call the method under test
		err := storage.AddOrder(ctx, order)

		// Assert results
		assert.NoError(t, err)
	})

	t.Run("transaction begin error", func(t *testing.T) {
		expectedError := errors.New("db error")
		mockDB.EXPECT().BeginTx(ctx).Return(nil, expectedError)

		err := storage.AddOrder(ctx, Order{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
	})

	t.Run("order creation error", func(t *testing.T) {
		expectedError := errors.New("repository error")

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).Return(expectedError)
		mockTx.EXPECT().Rollback(ctx).Return(nil)

		err := storage.AddOrder(ctx, Order{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add order")
	})

	t.Run("history entry creation error", func(t *testing.T) {
		expectedError := errors.New("history repository error")

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).Return(nil)
		mockHistoryRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).Return(expectedError)
		mockTx.EXPECT().Rollback(ctx).Return(nil)

		err := storage.AddOrder(ctx, Order{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add order history entry")
	})

}

func TestStorage_UpdateOrderStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockTx := mock_db.NewMockTx(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)
	storage.timeNow = func() time.Time { return fixedTime }

	t.Run("successful status update", func(t *testing.T) {
		orderID := "order-123"
		newStatus := "delivered"

		existingOrder := &repository.Order{
			ID:     orderID,
			Status: "shipped",
			// other fields omitted for brevity
		}

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().GetByID(ctx, orderID).Return(existingOrder, nil)
		mockOrderRepo.EXPECT().UpdateTx(ctx, mockTx, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ db.Tx, o *repository.Order) error {
				assert.Equal(t, orderID, o.ID)
				assert.Equal(t, newStatus, o.Status)
				return nil
			})
		mockHistoryRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ db.Tx, h *repository.HistoryEntry) error {
				assert.Equal(t, orderID, h.OrderID)
				assert.Equal(t, newStatus, h.Status)
				assert.Equal(t, fixedTime.UTC(), h.ChangedAt)
				return nil
			})
		mockTx.EXPECT().Commit(ctx).Return(nil)

		err := storage.UpdateOrderStatus(ctx, orderID, newStatus)

		assert.NoError(t, err)
	})

	t.Run("order not found", func(t *testing.T) {
		orderID := "non-existent-order"
		newStatus := "delivered"

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().GetByID(ctx, orderID).Return(nil, repository.ErrObjectNotFound)
		mockTx.EXPECT().Rollback(ctx).Return(nil)

		err := storage.UpdateOrderStatus(ctx, orderID, newStatus)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

func TestStorage_GetUserOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)

	t.Run("successful orders retrieval", func(t *testing.T) {
		userID := "user-123"
		limit := 10
		activeOnly := true

		time1 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		time2 := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)

		repoOrders := []*repository.Order{
			{
				ID:          "order-1",
				RecipientID: userID,
				Status:      "active",
				Wrapper:     "box",
				CreatedAt:   time1,
				UpdatedAt:   time1,
			},
			{
				ID:          "order-2",
				RecipientID: userID,
				Status:      "processing",
				Wrapper:     "envelope",
				CreatedAt:   time2,
				UpdatedAt:   time2,
			},
		}

		mockOrderRepo.EXPECT().GetByUserID(ctx, userID, limit, activeOnly).Return(repoOrders, nil)

		orders, err := storage.GetUserOrders(ctx, userID, limit, activeOnly)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(orders))
		assert.Equal(t, "order-1", orders[0].ID)
		assert.Equal(t, "order-2", orders[1].ID)
		assert.Equal(t, Container("box"), orders[0].Wrapper)
		assert.Equal(t, Container("envelope"), orders[1].Wrapper)
	})

	t.Run("repository error", func(t *testing.T) {
		userID := "user-123"
		limit := 10
		activeOnly := false
		expectedError := errors.New("database error")

		mockOrderRepo.EXPECT().GetByUserID(ctx, userID, limit, activeOnly).Return(nil, expectedError)

		orders, err := storage.GetUserOrders(ctx, userID, limit, activeOnly)

		assert.Error(t, err)
		assert.Nil(t, orders)
		assert.Contains(t, err.Error(), "failed to get user orders")
	})

	t.Run("empty results", func(t *testing.T) {
		userID := "user-no-orders"
		limit := 10
		activeOnly := true

		mockOrderRepo.EXPECT().GetByUserID(ctx, userID, limit, activeOnly).Return([]*repository.Order{}, nil)

		orders, err := storage.GetUserOrders(ctx, userID, limit, activeOnly)

		assert.NoError(t, err)
		assert.Empty(t, orders)
	})
}

func TestStorage_GetOrderHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)

	t.Run("successful history retrieval", func(t *testing.T) {
		orderID := "order-123"
		time1 := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		time2 := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)

		repoEntries := []*repository.HistoryEntry{
			{
				OrderID:   orderID,
				Status:    "created",
				ChangedAt: time1,
			},
			{
				OrderID:   orderID,
				Status:    "processing",
				ChangedAt: time2,
			},
		}

		mockHistoryRepo.EXPECT().GetByOrderID(ctx, orderID).Return(repoEntries, nil)

		history, err := storage.GetOrderHistory(ctx, orderID)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(history))
		assert.Equal(t, "created", history[0].Status)
		assert.Equal(t, "processing", history[1].Status)
		assert.Equal(t, time1, history[0].ChangedAt)
		assert.Equal(t, time2, history[1].ChangedAt)
	})

	t.Run("repository error", func(t *testing.T) {
		orderID := "order-123"
		expectedError := errors.New("database error")

		mockHistoryRepo.EXPECT().GetByOrderID(ctx, orderID).Return(nil, expectedError)

		history, err := storage.GetOrderHistory(ctx, orderID)

		assert.Error(t, err)
		assert.Nil(t, history)
		assert.Contains(t, err.Error(), "failed to get order history")
	})

	t.Run("empty history", func(t *testing.T) {
		orderID := "order-new"

		mockHistoryRepo.EXPECT().GetByOrderID(ctx, orderID).Return([]*repository.HistoryEntry{}, nil)

		history, err := storage.GetOrderHistory(ctx, orderID)

		assert.NoError(t, err)
		assert.Empty(t, history)
	})
}

func TestStorage_AddReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockTx := mock_db.NewMockTx(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)

	t.Run("successful return creation", func(t *testing.T) {
		returnTime := time.Date(2023, 1, 10, 12, 0, 0, 0, time.UTC)
		ret := Return{
			OrderID:    "order-123",
			UserID:     "user-456",
			ReturnedAt: returnTime,
		}

		expectedRepoReturn := &repository.ReturnEntry{
			OrderID:    ret.OrderID,
			UserID:     ret.UserID,
			ReturnedAt: ret.ReturnedAt,
		}

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockReturnRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ db.Tx, r *repository.ReturnEntry) error {
				assert.Equal(t, expectedRepoReturn.OrderID, r.OrderID)
				assert.Equal(t, expectedRepoReturn.UserID, r.UserID)
				assert.Equal(t, expectedRepoReturn.ReturnedAt, r.ReturnedAt)
				return nil
			})
		mockTx.EXPECT().Commit(ctx).Return(nil)

		err := storage.AddReturn(ctx, ret)

		assert.NoError(t, err)
	})

	t.Run("transaction begin error", func(t *testing.T) {
		expectedError := errors.New("db error")
		mockDB.EXPECT().BeginTx(ctx).Return(nil, expectedError)

		err := storage.AddReturn(ctx, Return{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
	})

	t.Run("return creation error", func(t *testing.T) {
		expectedError := errors.New("repository error")

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockReturnRepo.EXPECT().CreateTx(ctx, mockTx, gomock.Any()).Return(expectedError)
		mockTx.EXPECT().Rollback(ctx).Return(nil)

		err := storage.AddReturn(ctx, Return{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add return")
	})

}

func TestStorage_GetReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)

	t.Run("successful returns retrieval", func(t *testing.T) {
		page := 1
		limit := 10
		returnTime1 := time.Date(2023, 1, 10, 12, 0, 0, 0, time.UTC)
		returnTime2 := time.Date(2023, 1, 11, 12, 0, 0, 0, time.UTC)

		repoReturns := []*repository.ReturnEntry{
			{
				OrderID:    "order-1",
				UserID:     "user-1",
				ReturnedAt: returnTime1,
			},
			{
				OrderID:    "order-2",
				UserID:     "user-2",
				ReturnedAt: returnTime2,
			},
		}

		mockReturnRepo.EXPECT().GetPaginated(ctx, page, limit).Return(repoReturns, nil)

		returns, err := storage.GetReturns(ctx, page, limit)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(returns))
		assert.Equal(t, "order-1", returns[0].OrderID)
		assert.Equal(t, "user-1", returns[0].UserID)
		assert.Equal(t, returnTime1, returns[0].ReturnedAt)
		assert.Equal(t, "order-2", returns[1].OrderID)
		assert.Equal(t, "user-2", returns[1].UserID)
		assert.Equal(t, returnTime2, returns[1].ReturnedAt)
	})

	t.Run("repository error", func(t *testing.T) {
		page := 1
		limit := 10
		expectedError := errors.New("database error")

		mockReturnRepo.EXPECT().GetPaginated(ctx, page, limit).Return(nil, expectedError)

		returns, err := storage.GetReturns(ctx, page, limit)

		assert.Error(t, err)
		assert.Nil(t, returns)
		assert.Contains(t, err.Error(), "failed to get returns")
	})

	t.Run("empty results", func(t *testing.T) {
		page := 5
		limit := 10

		mockReturnRepo.EXPECT().GetPaginated(ctx, page, limit).Return([]*repository.ReturnEntry{}, nil)

		returns, err := storage.GetReturns(ctx, page, limit)

		assert.NoError(t, err)
		assert.Empty(t, returns)
	})
}

func TestStorage_DeleteOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockTx := mock_db.NewMockTx(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()
	storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)

	t.Run("successful order deletion", func(t *testing.T) {
		orderID := "order-123"

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().DeleteTx(ctx, mockTx, orderID).Return(nil)
		mockTx.EXPECT().Commit(ctx).Return(nil)

		err := storage.DeleteOrder(ctx, orderID)

		assert.NoError(t, err)
	})

	t.Run("transaction begin error", func(t *testing.T) {
		orderID := "order-123"
		expectedError := errors.New("db error")

		mockDB.EXPECT().BeginTx(ctx).Return(nil, expectedError)

		err := storage.DeleteOrder(ctx, orderID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
	})

	t.Run("order deletion error", func(t *testing.T) {
		orderID := "order-123"
		expectedError := errors.New("repository error")

		mockDB.EXPECT().BeginTx(ctx).Return(mockTx, nil)
		mockOrderRepo.EXPECT().DeleteTx(ctx, mockTx, orderID).Return(expectedError)
		mockTx.EXPECT().Rollback(ctx).Return(nil)

		err := storage.DeleteOrder(ctx, orderID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete order")
	})

}

func TestNewStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_db.NewMockDB(ctrl)
	mockOrderRepo := mock_storage.NewMockOrderRepository(ctrl)
	mockHistoryRepo := mock_storage.NewMockHistoryRepository(ctrl)
	mockReturnRepo := mock_storage.NewMockReturnRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	ctx := context.Background()

	t.Run("successful storage creation", func(t *testing.T) {
		storage := NewStorage(ctx, mockDB, mockOrderRepo, mockReturnRepo, mockHistoryRepo, mockUserRepo)

		assert.NotNil(t, storage)
		assert.Equal(t, mockDB, storage.db)
		assert.Equal(t, mockOrderRepo, storage.orderRepo)
		assert.Equal(t, mockReturnRepo, storage.returnRepo)
		assert.Equal(t, mockHistoryRepo, storage.historyRepo)
		assert.Equal(t, mockUserRepo, storage.userRepo)
		assert.NotNil(t, storage.timeNow)
	})
}
