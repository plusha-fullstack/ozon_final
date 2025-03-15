package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mock_server "gitlab.ozon.dev/pupkingeorgij/homework/internal/server/mocks"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

func TestHandleCreateOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mock_server.NewMockStorage(ctrl)
	mockUserRepo := mock_server.NewMockUserRepo(ctrl)
	server := New(mockStorage, mockUserRepo)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful order creation",
			requestBody: map[string]interface{}{
				"recipient_id":  "user123",
				"storage_until": time.Now().Add(24 * time.Hour).Format("2006-01-02"),
				"price":         100,
				"weight":        2.5,
				"wrapper":       storage.Container("Box"),
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					AddOrder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, order storage.Order) error {
						assert.Equal(t, "user123", order.RecipientID)
						assert.Equal(t, 100, order.Price)
						assert.Equal(t, float32(2.5), order.Weight)
						t.Log("i am here", storage.Box, order.Wrapper)
						assert.Equal(t, string(storage.Box), string(order.Wrapper))
						return nil
					})
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"message":"Order created successfully"}`,
		},
		{
			name: "invalid request body",
			requestBody: map[string]interface{}{
				"price": 100,
			},
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Invalid request body"}`,
		},
		{
			name: "storage error",
			requestBody: map[string]interface{}{
				"recipient_id":  "user123",
				"storage_until": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				"price":         100,
				"weight":        2.5,
				"wrapper":       "Box",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					AddOrder(gomock.Any(), gomock.Any()).
					Return(errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"Failed to create order"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			body, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			server.handleCreateOrder(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.JSONEq(t, tc.expectedBody, rr.Body.String())
		})
	}
}

func TestHandleGetOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mock_server.NewMockStorage(ctrl)
	mockUserRepo := mock_server.NewMockUserRepo(ctrl)
	server := New(mockStorage, mockUserRepo)

	tests := []struct {
		name           string
		orderID        string
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "order found",
			orderID: "order123",
			setupMocks: func() {
				order := &storage.Order{
					ID:           "order123",
					RecipientID:  "user123",
					StorageUntil: time.Now().Add(24 * time.Hour),
					Status:       "pending",
					Price:        100,
					Weight:       2.5,
					Wrapper:      storage.Box,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				mockStorage.EXPECT().
					GetOrder(gomock.Any(), "order123").
					Return(order, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"id":"order123"`,
		},
		{
			name:    "order not found",
			orderID: "nonexistent",
			setupMocks: func() {
				mockStorage.EXPECT().
					GetOrder(gomock.Any(), "nonexistent").
					Return(nil, errors.New("order not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"Order not found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			req := httptest.NewRequest(http.MethodGet, "/orders/"+tc.orderID, nil)
			req = mux.SetURLVars(req, map[string]string{"id": tc.orderID})

			rr := httptest.NewRecorder()

			server.handleGetOrder(rr, req)
			assert.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectedStatus == http.StatusOK {
				assert.Contains(t, rr.Body.String(), tc.expectedBody)
			} else {
				assert.JSONEq(t, tc.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestHandleUpdateOrderStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mock_server.NewMockStorage(ctrl)
	mockUserRepo := mock_server.NewMockUserRepo(ctrl)
	server := New(mockStorage, mockUserRepo)

	tests := []struct {
		name           string
		orderID        string
		requestBody    map[string]interface{}
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "successful status update",
			orderID: "order123",
			requestBody: map[string]interface{}{
				"status": "delivered",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					UpdateOrderStatus(gomock.Any(), "order123", "delivered").
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"message":"Order status updated successfully"}`,
		},
		{
			name:           "invalid request body",
			orderID:        "order123",
			requestBody:    map[string]interface{}{},
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Invalid request body"}`,
		},
		{
			name:    "storage error",
			orderID: "order123",
			requestBody: map[string]interface{}{
				"status": "delivered",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					UpdateOrderStatus(gomock.Any(), "order123", "delivered").
					Return(errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"Failed to update order status"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			body, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPut, "/orders/"+tc.orderID+"/status", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": tc.orderID})

			rr := httptest.NewRecorder()

			server.handleUpdateOrderStatus(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.JSONEq(t, tc.expectedBody, rr.Body.String())
		})
	}
}

func TestHandleListOrders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mock_server.NewMockStorage(ctrl)
	mockUserRepo := mock_server.NewMockUserRepo(ctrl)
	server := New(mockStorage, mockUserRepo)

	tests := []struct {
		name           string
		userID         string
		queryParams    map[string]string
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "successful orders listing",
			userID: "user123",
			queryParams: map[string]string{
				"last_n":      "5",
				"active_only": "true",
			},
			setupMocks: func() {
				orders := []storage.Order{
					{
						ID:           "order1",
						RecipientID:  "user123",
						StorageUntil: time.Now().Add(24 * time.Hour),
						Status:       "pending",
						Price:        100,
						Weight:       2.5,
						Wrapper:      storage.Box,
					},
					{
						ID:           "order2",
						RecipientID:  "user123",
						StorageUntil: time.Now().Add(48 * time.Hour),
						Status:       "processing",
						Price:        150,
						Weight:       3.0,
						Wrapper:      storage.Bag,
					},
				}
				mockStorage.EXPECT().
					GetUserOrders(gomock.Any(), "user123", 5, true).
					Return(orders, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"orders":[{"id":"order1"`,
		},
		{
			name:   "invalid last_n parameter",
			userID: "user123",
			queryParams: map[string]string{
				"last_n":      "invalid",
				"active_only": "true",
			},
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Invalid last_n parameter"}`,
		},
		{
			name:   "storage error",
			userID: "user123",
			queryParams: map[string]string{
				"last_n":      "5",
				"active_only": "true",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					GetUserOrders(gomock.Any(), "user123", 5, true).
					Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"Failed to get user orders"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			req := httptest.NewRequest(http.MethodGet, "/users/"+tc.userID+"/orders", nil)
			q := req.URL.Query()
			for k, v := range tc.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			req = mux.SetURLVars(req, map[string]string{"user_id": tc.userID})

			rr := httptest.NewRecorder()

			server.handleListOrders(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectedStatus == http.StatusOK {
				assert.Contains(t, rr.Body.String(), tc.expectedBody)
			} else {
				assert.JSONEq(t, tc.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestHandleAddReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mock_server.NewMockStorage(ctrl)
	mockUserRepo := mock_server.NewMockUserRepo(ctrl)
	server := New(mockStorage, mockUserRepo)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		setupMocks     func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful return creation",
			requestBody: map[string]interface{}{
				"order_id": "order123",
				"user_id":  "user123",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					GetOrder(gomock.Any(), "order123").
					Return(&storage.Order{
						ID:          "order123",
						RecipientID: "user123",
						Status:      "delivered",
					}, nil)

				mockStorage.EXPECT().
					AddReturn(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, ret storage.Return) error {
						assert.Equal(t, "order123", ret.OrderID)
						assert.Equal(t, "user123", ret.UserID)
						assert.False(t, ret.ReturnedAt.IsZero())
						return nil
					})

				mockStorage.EXPECT().
					UpdateOrderStatus(gomock.Any(), "order123", "returned").
					Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"message":"Return created successfully"}`,
		},
		{
			name:           "invalid request body",
			requestBody:    map[string]interface{}{},
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Invalid request body"}`,
		},
		{
			name: "order not found",
			requestBody: map[string]interface{}{
				"order_id": "nonexistent",
				"user_id":  "user123",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					GetOrder(gomock.Any(), "nonexistent").
					Return(nil, errors.New("order not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"Order not found"}`,
		},
		{
			name: "user mismatch",
			requestBody: map[string]interface{}{
				"order_id": "order123",
				"user_id":  "wronguser",
			},
			setupMocks: func() {
				mockStorage.EXPECT().
					GetOrder(gomock.Any(), "order123").
					Return(&storage.Order{
						ID:          "order123",
						RecipientID: "user123",
						Status:      "delivered",
					}, nil)
			},
			expectedStatus: http.StatusForbidden,
			expectedBody:   `{"error":"User ID does not match the order recipient"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			body, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/returns", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			server.handleAddReturn(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.JSONEq(t, tc.expectedBody, rr.Body.String())
		})
	}
}
