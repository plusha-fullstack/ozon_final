package postgresql_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	mock_database "gitlab.ozon.dev/pupkingeorgij/homework/internal/db/mocks"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository/postgresql"
	"go.uber.org/mock/gomock"
)

func TestOrderRepo_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		now := changed.UTC()
		testOrder := &repository.Order{
			ID:           "order-123",
			RecipientID:  "user-456",
			StorageUntil: now.Add(24 * time.Hour),
			Status:       "pending",
			Price:        100.0,
			Weight:       2.5,
			Wrapper:      "standard",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		mockDB.EXPECT().Exec(
			gomock.Any(),
			gomock.Any(),
			gomock.Eq(testOrder.ID),
			gomock.Eq(testOrder.RecipientID),
			gomock.Eq(testOrder.StorageUntil),
			gomock.Eq(testOrder.Status),
			gomock.Eq(testOrder.Price),
			gomock.Eq(testOrder.Weight),
			gomock.Eq(testOrder.Wrapper),
			gomock.Eq(testOrder.CreatedAt),
			gomock.Eq(testOrder.UpdatedAt),
		).Return(nil, nil)

		err = repo.Create(ctx, testOrder)
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		expectedErr := errors.New("database error")
		testOrder := &repository.Order{
			ID: "order-123",
		}

		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		err := repo.Create(ctx, testOrder)
		assert.Equal(t, expectedErr, err)
	})
}

func TestOrderRepo_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("order found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		now := changed.UTC()
		testOrder := &repository.Order{
			ID:           "order-123",
			RecipientID:  "user-456",
			StorageUntil: now.Add(24 * time.Hour),
			Status:       "pending",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		mockDB.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(testOrder.ID)).
			DoAndReturn(func(_ context.Context, dest *repository.Order, _ string, _ string) error {
				*dest = *testOrder
				return nil
			})

		order, err := repo.GetByID(ctx, testOrder.ID)
		assert.NoError(t, err)
		assert.Equal(t, testOrder, order)
	})

	t.Run("order not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		mockDB.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(pgx.ErrNoRows)

		order, err := repo.GetByID(ctx, "non-existent-id")
		assert.ErrorIs(t, err, repository.ErrObjectNotFound)
		assert.Nil(t, order)
	})

	t.Run("database error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		expectedErr := errors.New("database error")
		mockDB.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(expectedErr)

		order, err := repo.GetByID(ctx, "order-123")
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, order)
	})
}

func TestOrderRepo_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		now := changed.UTC()
		testOrder := &repository.Order{
			ID:           "order-123",
			RecipientID:  "user-456",
			StorageUntil: now.Add(24 * time.Hour),
			Status:       "completed",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		mockDB.EXPECT().Exec(
			gomock.Any(),
			gomock.Any(),
			gomock.Eq(testOrder.RecipientID),
			gomock.Eq(testOrder.StorageUntil),
			gomock.Eq(testOrder.Status),
			gomock.Eq(testOrder.Price),
			gomock.Eq(testOrder.Weight),
			gomock.Eq(testOrder.Wrapper),
			gomock.Eq(testOrder.UpdatedAt),
			gomock.Eq(testOrder.ID),
		).Return(nil, nil)

		err = repo.Update(ctx, testOrder)
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		expectedErr := errors.New("database error")
		testOrder := &repository.Order{
			ID: "order-123",
		}

		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, expectedErr)

		err := repo.Update(ctx, testOrder)
		assert.Equal(t, expectedErr, err)
	})
}

func TestOrderRepo_GetByUserID(t *testing.T) {
	ctx := context.Background()

	t.Run("get all orders with limit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		userID := "user-456"
		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		now := changed.UTC()
		testOrders := []*repository.Order{
			{
				ID:           "order-123",
				RecipientID:  userID,
				StorageUntil: now.Add(24 * time.Hour),
				Status:       "pending",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "order-124",
				RecipientID:  userID,
				StorageUntil: now.Add(48 * time.Hour),
				Status:       "in_progress",
				CreatedAt:    now.Add(1 * time.Hour),
				UpdatedAt:    now.Add(1 * time.Hour),
			},
		}

		mockDB.EXPECT().Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(userID), gomock.Eq(10)).
			DoAndReturn(func(_ context.Context, dest *[]*repository.Order, _ string, _ string, _ int) error {
				*dest = testOrders
				return nil
			})

		orders, err := repo.GetByUserID(ctx, userID, 10, false)
		assert.NoError(t, err)
		assert.Equal(t, testOrders, orders)
	})

	t.Run("get active orders only", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		userID := "user-456"
		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		now := changed.UTC()
		testOrders := []*repository.Order{
			{
				ID:           "order-123",
				RecipientID:  userID,
				StorageUntil: now.Add(24 * time.Hour),
				Status:       "pending",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}

		mockDB.EXPECT().Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(userID)).
			DoAndReturn(func(_ context.Context, dest *[]*repository.Order, _ string, _ string) error {
				*dest = testOrders
				return nil
			})

		orders, err := repo.GetByUserID(ctx, userID, 0, true)
		assert.NoError(t, err)
		assert.Equal(t, testOrders, orders)
	})

	t.Run("database error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		userID := "user-456"
		expectedErr := errors.New("database error")

		mockDB.EXPECT().Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(expectedErr)

		orders, err := repo.GetByUserID(ctx, userID, 0, false)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, orders)
	})
}

func TestOrderRepo_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		orderID := "order-123"

		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Eq(orderID)).
			Return(nil, nil)

		err := repo.Delete(ctx, orderID)
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		orderID := "order-123"
		expectedErr := errors.New("database error")

		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Eq(orderID)).
			Return(nil, expectedErr)

		err := repo.Delete(ctx, orderID)
		assert.Equal(t, expectedErr, err)
	})
}

func TestOrderRepo_CreateTx(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		mockTx := mock_database.NewMockTx(ctrl)
		repo := postgresql.NewOrderRepo(mockDB)

		changed, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		now := changed.UTC()
		testOrder := &repository.Order{
			ID:           "order-123",
			RecipientID:  "user-456",
			StorageUntil: now.Add(24 * time.Hour),
			Status:       "pending",
			Price:        100.0,
			Weight:       2.5,
			Wrapper:      "standard",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		mockTx.EXPECT().Exec(
			gomock.Any(),
			gomock.Any(),
			gomock.Eq(testOrder.ID),
			gomock.Eq(testOrder.RecipientID),
			gomock.Eq(testOrder.StorageUntil),
			gomock.Eq(testOrder.Status),
			gomock.Eq(testOrder.Price),
			gomock.Eq(testOrder.Weight),
			gomock.Eq(testOrder.Wrapper),
			gomock.Eq(testOrder.CreatedAt),
			gomock.Eq(testOrder.UpdatedAt),
		).Return(nil, nil)

		err = repo.CreateTx(ctx, mockTx, testOrder)
		assert.NoError(t, err)
	})
}
