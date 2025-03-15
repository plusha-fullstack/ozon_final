package postgresql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository/postgresql"
	"go.uber.org/mock/gomock"

	mock_database "gitlab.ozon.dev/pupkingeorgij/homework/internal/db/mocks"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
)

func TestOrderRepo_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := postgresql.NewOrderRepo(mockDB)

	ctx := context.Background()
	now := time.Now()
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

	t.Run("success", func(t *testing.T) {
		mockDB.EXPECT().Exec(
			gomock.Any(),
			gomock.Any(),
			testOrder.ID,
			testOrder.RecipientID,
			testOrder.StorageUntil,
			testOrder.Status,
			testOrder.Price,
			testOrder.Weight,
			testOrder.Wrapper,
			testOrder.CreatedAt,
			testOrder.UpdatedAt,
		).Return(nil, nil)

		err := repo.Create(ctx, testOrder)
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		err := repo.Create(ctx, testOrder)
		assert.Equal(t, expectedErr, err)
	})
}

func TestOrderRepo_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := postgresql.NewOrderRepo(mockDB)

	ctx := context.Background()
	now := time.Now()
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

	t.Run("order found", func(t *testing.T) {
		mockDB.EXPECT().Get(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			testOrder.ID,
		).DoAndReturn(func(_ context.Context, dest *repository.Order, _ string, _ string) error {
			*dest = *testOrder
			return nil
		})

		order, err := repo.GetByID(ctx, testOrder.ID)
		assert.NoError(t, err)
		assert.Equal(t, testOrder, order)
	})

	t.Run("order not found", func(t *testing.T) {
		mockDB.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(pgx.ErrNoRows)

		order, err := repo.GetByID(ctx, "non-existent-id")
		assert.ErrorIs(t, err, repository.ErrObjectNotFound)
		assert.Nil(t, order)
	})

	t.Run("database error", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockDB.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr)

		order, err := repo.GetByID(ctx, testOrder.ID)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, order)
	})
}

func TestOrderRepo_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := postgresql.NewOrderRepo(mockDB)

	ctx := context.Background()
	now := time.Now()
	testOrder := &repository.Order{
		ID:           "order-123",
		RecipientID:  "user-456",
		StorageUntil: now.Add(24 * time.Hour),
		Status:       "completed",
		Price:        100.0,
		Weight:       2.5,
		Wrapper:      "standard",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	t.Run("success", func(t *testing.T) {
		mockDB.EXPECT().Exec(
			gomock.Any(),
			gomock.Any(),
			testOrder.RecipientID,
			testOrder.StorageUntil,
			testOrder.Status,
			testOrder.Price,
			testOrder.Weight,
			testOrder.Wrapper,
			testOrder.UpdatedAt,
			testOrder.ID,
		).Return(nil, nil)

		err := repo.Update(ctx, testOrder)
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		err := repo.Update(ctx, testOrder)
		assert.Equal(t, expectedErr, err)
	})
}

func TestOrderRepo_GetByUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := postgresql.NewOrderRepo(mockDB)

	ctx := context.Background()
	userID := "user-456"
	now := time.Now()
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

	t.Run("get all orders with limit", func(t *testing.T) {
		limit := 10
		mockDB.EXPECT().Select(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			userID,
			limit,
		).DoAndReturn(func(_ context.Context, dest *[]*repository.Order, _ string, _ string, _ int) error {
			*dest = testOrders
			return nil
		})

		orders, err := repo.GetByUserID(ctx, userID, limit, false)
		assert.NoError(t, err)
		assert.Equal(t, testOrders, orders)
	})

	t.Run("get active orders only", func(t *testing.T) {
		limit := 0 // no limit
		mockDB.EXPECT().Select(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			userID,
		).DoAndReturn(func(_ context.Context, dest *[]*repository.Order, _ string, _ string) error {
			*dest = testOrders
			return nil
		})

		orders, err := repo.GetByUserID(ctx, userID, limit, true)
		assert.NoError(t, err)
		assert.Equal(t, testOrders, orders)
	})

	t.Run("database error", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockDB.EXPECT().Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr)

		orders, err := repo.GetByUserID(ctx, userID, 0, false)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, orders)
	})
}

func TestOrderRepo_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := postgresql.NewOrderRepo(mockDB)

	ctx := context.Background()
	orderID := "order-123"

	t.Run("success", func(t *testing.T) {
		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), orderID).Return(nil, nil)

		err := repo.Delete(ctx, orderID)
		assert.NoError(t, err)
	})

	t.Run("database error", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockDB.EXPECT().Exec(gomock.Any(), gomock.Any(), orderID).Return(nil, expectedErr)

		err := repo.Delete(ctx, orderID)
		assert.Equal(t, expectedErr, err)
	})
}

func TestOrderRepo_CreateTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	mockTx := mock_database.NewMockTx(ctrl)
	repo := postgresql.NewOrderRepo(mockDB)

	ctx := context.Background()
	now := time.Now()
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
		testOrder.ID,
		testOrder.RecipientID,
		testOrder.StorageUntil,
		testOrder.Status,
		testOrder.Price,
		testOrder.Weight,
		testOrder.Wrapper,
		testOrder.CreatedAt,
		testOrder.UpdatedAt,
	).Return(nil, nil)

	err := repo.CreateTx(ctx, mockTx, testOrder)
	assert.NoError(t, err)
}
