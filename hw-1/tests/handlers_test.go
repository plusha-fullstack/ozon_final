//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/server"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
	"gitlab.ozon.dev/pupkingeorgij/homework/tests/collections"
	"gitlab.ozon.dev/pupkingeorgij/homework/tests/postgresql"
)

var (
	db  *postgresql.TDB
	srv *server.Server
)

type mockUserRepo struct{}

func (m *mockUserRepo) ValidateUser(ctx context.Context, username, password string) (bool, error) {
	return true, nil
}

func init() {
	db = postgresql.NewFromEnv()

	storage := storage.NewPostgresStorage(db.DB)
	srv = server.New(storage, &mockUserRepo{})
}

func TestCreateOrder(t *testing.T) {
	tdb.SetUp(t, "orders_fixture", "order_history_fixture", "returns_fixture", "users_fixture")
	defer db.TearDown(t)

	order := collections.Order1
	orderJSON, err := json.Marshal(order)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("test", "test")

	w := httptest.NewRecorder()
	http.HandlerFunc(srv.HandleCreateOrder).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	createdOrder, err := storage.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, createdOrder.ID)
	assert.Equal(t, order.RecipientID, createdOrder.RecipientID)
	assert.Equal(t, order.Price, createdOrder.Price)
}

func TestCreateOrderInvalidData(t *testing.T) {
	tdb.SetUp(t, "orders_fixture", "order_history_fixture", "returns_fixture", "users_fixture")
	defer db.TearDown(t)

	invalidOrder := struct {
		ID string `json:"id"`
	}{
		ID: "invalid-order",
	}

	orderJSON, err := json.Marshal(invalidOrder)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("test", "test")

	w := httptest.NewRecorder()
	http.HandlerFunc(srv.HandleCreateOrder).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetOrder(t *testing.T) {
	tdb.SetUp(t, "orders_fixture", "order_history_fixture", "returns_fixture", "users_fixture")
	defer db.TearDown(t)

	req := httptest.NewRequest(http.MethodGet, "/orders/order1", nil)
	req.SetBasicAuth("test", "test")

	w := httptest.NewRecorder()
	req = req.WithContext(context.WithValue(req.Context(), "orderID", "order1"))

	http.HandlerFunc(srv.HandleGetOrder).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var order storage.Order
	err := json.Unmarshal(w.Body.Bytes(), &order)
	require.NoError(t, err)

	assert.Equal(t, "order1", order.ID)
	assert.Equal(t, "user1", order.RecipientID)
}

func TestUpdateOrderStatus(t *testing.T) {
	tdb.SetUp(t, "orders_fixture", "order_history_fixture", "returns_fixture", "users_fixture")
	defer db.TearDown(t)

	statusUpdate := struct {
		Status string `json:"status"`
	}{
		Status: "shipped",
	}

	statusJSON, err := json.Marshal(statusUpdate)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/orders/order1/", bytes.NewBuffer(statusJSON))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("test", "test")

	w := httptest.NewRecorder()
	req = req.WithContext(context.WithValue(req.Context(), "orderID", "order1"))

	http.HandlerFunc(srv.HandleUpdateOrderStatus).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	updatedOrder, err := storage.GetOrder(context.Background(), "order1")
	require.NoError(t, err)
	assert.Equal(t, "returned", updatedOrder.Status)
}
