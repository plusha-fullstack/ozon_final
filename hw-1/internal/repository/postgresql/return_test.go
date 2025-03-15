package postgresql

import (
	"context"
	"errors"
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := NewReturnRepo(mockDB)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: time.Now(),
		}

		mockDB.EXPECT().
			Exec(gomock.Any(), gomock.Any(),
				gomock.Eq(entry.OrderID),
				gomock.Eq(entry.UserID),
				gomock.Eq(entry.ReturnedAt)).
			Return(nil, nil)

		err := repo.Create(ctx, entry)
		assert.NoError(t, err)
	})

	t.Run("DB Error", func(t *testing.T) {
		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: time.Now(),
		}
		dbErr := errors.New("database error")

		mockDB.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, dbErr)

		err := repo.Create(ctx, entry)
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
	})
}

func TestReturnRepo_CreateTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	mockTx := mock_database.NewMockTx(ctrl)
	repo := NewReturnRepo(mockDB)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: time.Now(),
		}

		mockTx.EXPECT().
			Exec(gomock.Any(), gomock.Any(),
				gomock.Eq(entry.OrderID),
				gomock.Eq(entry.UserID),
				gomock.Eq(entry.ReturnedAt)).
			Return(nil, nil)

		err := repo.CreateTx(ctx, mockTx, entry)
		assert.NoError(t, err)
	})

	t.Run("Tx Error", func(t *testing.T) {
		entry := &repository.ReturnEntry{
			OrderID:    "order123",
			UserID:     "user456",
			ReturnedAt: time.Now(),
		}
		txErr := errors.New("transaction error")

		mockTx.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, txErr)

		err := repo.CreateTx(ctx, mockTx, entry)
		assert.Error(t, err)
		assert.Equal(t, txErr, err)
	})
}

func TestReturnRepo_GetPaginated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDB(ctrl)
	repo := NewReturnRepo(mockDB)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		page := 1
		limit := 10
		offset := (page - 1) * limit

		expectedReturns := []*repository.ReturnEntry{
			{
				ID:         1,
				OrderID:    "order123",
				UserID:     "user456",
				ReturnedAt: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:         2,
				OrderID:    "order789",
				UserID:     "user456",
				ReturnedAt: time.Now().Add(-1 * time.Hour),
			},
		}

		mockDB.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(limit), gomock.Eq(offset)).
			DoAndReturn(func(_ context.Context, dest interface{}, _ string, _ ...interface{}) error {
				returns := dest.(*[]*repository.ReturnEntry)
				*returns = expectedReturns
				return nil
			})

		returns, err := repo.GetPaginated(ctx, page, limit)
		assert.NoError(t, err)
		assert.Equal(t, expectedReturns, returns)
	})

	t.Run("Empty Result", func(t *testing.T) {
		page := 100
		limit := 10
		offset := (page - 1) * limit
		var emptyReturns []*repository.ReturnEntry

		mockDB.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(limit), gomock.Eq(offset)).
			DoAndReturn(func(_ context.Context, dest interface{}, _ string, _ ...interface{}) error {
				returns := dest.(*[]*repository.ReturnEntry)
				*returns = emptyReturns
				return nil
			})

		returns, err := repo.GetPaginated(ctx, page, limit)
		assert.NoError(t, err)
		assert.Empty(t, returns)
	})

	t.Run("DB Error", func(t *testing.T) {
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
