package postgresql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock_database "gitlab.ozon.dev/pupkingeorgij/homework/internal/db/mocks"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository"
	"go.uber.org/mock/gomock"
)

func TestMain(m *testing.M) {
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "postgres")
	os.Setenv("DB_NAME", "test_db")

	code := m.Run()
	os.Exit(code)
}

func TestReturnRepo_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewReturnRepo(mockDB)

		returnedAt, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: returnedAt.UTC(),
		}

		mockDB.EXPECT().
			Exec(gomock.Any(), gomock.Any(),
				gomock.Eq(entry.OrderID),
				gomock.Eq(entry.UserID),
				gomock.Eq(entry.ReturnedAt)).
			Return(nil, nil)

		err = repo.Create(ctx, entry)
		assert.NoError(t, err)
	})

	t.Run("DB Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewReturnRepo(mockDB)

		returnedAt, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: returnedAt.UTC(),
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

func TestReturnRepo_CreateTx(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		mockTx := mock_database.NewMockTx(ctrl)
		repo := NewReturnRepo(mockDB)

		returnedAt, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: returnedAt.UTC(),
		}

		mockTx.EXPECT().
			Exec(gomock.Any(), gomock.Any(),
				gomock.Eq(entry.OrderID),
				gomock.Eq(entry.UserID),
				gomock.Eq(entry.ReturnedAt)).
			Return(nil, nil)

		err = repo.CreateTx(ctx, mockTx, entry)
		assert.NoError(t, err)
	})

	t.Run("Tx Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		mockTx := mock_database.NewMockTx(ctrl)
		repo := NewReturnRepo(mockDB)

		returnedAt, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: returnedAt.UTC(),
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

func TestReturnRepo_GetPaginated(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewReturnRepo(mockDB)

		page := 1
		limit := 10

		returnedAt1, err := time.Parse("2006-01-02", "2025-01-01")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		returnedAt2, err := time.Parse("2006-01-02", "2025-01-02")
		if err != nil {
			fmt.Println("Error parsing date:", err)
			return
		}

		expectedReturns := []*repository.ReturnEntry{
			{
				ID:         1,
				OrderID:    "order123",
				UserID:     "user456",
				ReturnedAt: returnedAt1.UTC(),
			},
			{
				ID:         2,
				OrderID:    "order789",
				UserID:     "user456",
				ReturnedAt: returnedAt2.UTC(),
			},
		}

		returns, err := repo.GetPaginated(ctx, page, limit)
		assert.NoError(t, err)
		assert.Equal(t, expectedReturns, returns)
	})

	t.Run("Empty Result", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewReturnRepo(mockDB)

		page := 100
		limit := 10

		returns, err := repo.GetPaginated(ctx, page, limit)
		assert.NoError(t, err)
		assert.Empty(t, returns)
	})

	t.Run("DB Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDB(ctrl)
		repo := NewReturnRepo(mockDB)

		page := 1
		limit := 10
		dbErr := errors.New("database error")

		mockDB.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(dbErr)

		returns, err := repo.GetPaginated(ctx, page, limit)
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
		assert.Nil(t, returns)
	})
}
