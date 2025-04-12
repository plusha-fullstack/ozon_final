package grpcserver

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "gitlab.ozon.dev/pupkingeorgij/homework/internal/api"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/metrics"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

var _ pb.OrderServiceServer = (*Server)(nil)

type Server struct {
	pb.UnimplementedOrderServiceServer
	storage storage.Storage
	logger  *zap.Logger
}

func NewServer(st storage.Storage, logger *zap.Logger) *Server {
	return &Server{
		storage: st,
		logger:  logger,
	}
}

func toProtoOrder(o *storage.Order) *pb.Order {
	if o == nil {
		return nil
	}
	return &pb.Order{
		Id:           o.ID,
		RecipientId:  o.RecipientID,
		StorageUntil: timestamppb.New(o.StorageUntil),
		Status:       o.Status,
		Price:        int32(o.Price),
		Weight:       o.Weight,
		WrapperType:  string(o.Wrapper),
		CreatedAt:    timestamppb.New(o.CreatedAt),
		UpdatedAt:    timestamppb.New(o.UpdatedAt),
	}
}

func toProtoReturn(r *storage.Return) *pb.Return {
	if r == nil {
		return nil
	}
	return &pb.Return{
		OrderId:    r.OrderID,
		UserId:     r.UserID,
		ReturnedAt: timestamppb.New(r.ReturnedAt),
	}
}

func toProtoHistoryEntry(h *storage.HistoryEntry) *pb.HistoryEntry {
	if h == nil {
		return nil
	}
	return &pb.HistoryEntry{
		OrderId:   h.OrderID,
		Status:    h.Status,
		UpdatedAt: timestamppb.New(h.ChangedAt),
	}
}

func (s *Server) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "CreateOrder"), zap.String("order_id_req", req.Id), zap.String("recipient_id", req.RecipientId))
	l.Debug("RPC call received")

	storageUntil := req.GetStorageUntil().AsTime()
	if storageUntil.IsZero() {
		l.Warn("Validation failed: missing storage_until")
		metrics.OperationErrorsTotal.WithLabelValues("create_order").Inc()
		return nil, status.Error(codes.InvalidArgument, "storage_until is required and must be a valid timestamp")
	}
	if storageUntil.Before(time.Now()) {
		l.Warn("Validation failed: storage period is in the past")
		metrics.OperationErrorsTotal.WithLabelValues("create_order").Inc()
		return nil, status.Error(codes.InvalidArgument, "Error: storage period is in the past")
	}

	packager, err := storage.GetPackager(req.WrapperType, req.WithAdditionalMembrane)
	if err != nil {
		l.Warn("Validation failed: invalid packager", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("create_order").Inc()
		return nil, status.Errorf(codes.InvalidArgument, "Error: %v", err)
	}

	if err := packager.ValidateWeight(req.Weight); err != nil {
		l.Warn("Validation failed: invalid weight", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("create_order").Inc()
		return nil, status.Errorf(codes.InvalidArgument, "Validation Failed: %v", err)
	}

	adjustedPrice := packager.AdjustPrice(int(req.Price))

	order := storage.Order{
		ID:           req.Id,
		RecipientID:  req.RecipientId,
		StorageUntil: storageUntil.UTC(),
		Status:       "received",
		Price:        adjustedPrice,
		Weight:       req.Weight,
		Wrapper:      storage.Container(packager.GetType()),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.storage.AddOrder(ctx, order); err != nil {
		l.Error("Failed to add order to storage", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("create_order").Inc()

		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Errorf(codes.AlreadyExists, "failed to create order: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	metrics.OrdersCreatedTotal.Inc()
	l.Info("Order created successfully", zap.String("order_id", order.ID))
	return &pb.CreateOrderResponse{Id: order.ID}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "GetOrder"), zap.String("order_id", req.OrderId))
	l.Debug("RPC call received")

	if req.OrderId == "" {
		l.Warn("Invalid argument: missing order_id")
		metrics.OperationErrorsTotal.WithLabelValues("get_order").Inc()
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.storage.GetOrder(ctx, req.OrderId)
	if err != nil {
		l.Error("Failed to get order", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("get_order").Inc()
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "order not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}

	l.Info("Order retrieved successfully")
	return &pb.GetOrderResponse{Order: toProtoOrder(order)}, nil
}

func (s *Server) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest) (*emptypb.Empty, error) {
	l := s.logger.With(zap.String("rpc_method", "UpdateOrderStatus"), zap.String("order_id", req.OrderId), zap.String("status", req.Status))
	l.Debug("RPC call received")

	if req.OrderId == "" || req.Status == "" {
		l.Warn("Invalid argument: missing order_id or status")
		metrics.OperationErrorsTotal.WithLabelValues("update_order_status").Inc()
		return nil, status.Error(codes.InvalidArgument, "order_id and status are required")
	}

	err := s.storage.UpdateOrderStatus(ctx, req.OrderId, req.Status)
	if err != nil {
		l.Error("Failed to update order status", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("update_order_status").Inc()
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "order not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to update order status: %v", err)
	}

	l.Info("Order status updated successfully")
	return &emptypb.Empty{}, nil
}

func (s *Server) DeleteOrder(ctx context.Context, req *pb.DeleteOrderRequest) (*emptypb.Empty, error) {
	l := s.logger.With(zap.String("rpc_method", "DeleteOrder"), zap.String("order_id", req.OrderId))
	l.Debug("RPC call received")

	if req.OrderId == "" {
		l.Warn("Invalid argument: missing order_id")
		metrics.OperationErrorsTotal.WithLabelValues("delete_order").Inc()
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.storage.GetOrder(ctx, req.OrderId)
	if err != nil {
		l.Warn("Failed to find order for deletion check", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("delete_order").Inc()
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "order not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get order before deletion: %v", err)
	}

	if order.Status != "received" {
		l.Warn("Precondition failed: order not in 'received' status", zap.String("current_status", order.Status))
		metrics.OperationErrorsTotal.WithLabelValues("delete_order").Inc()
		return nil, status.Error(codes.FailedPrecondition, "Cannot return: order not in 'received' status")
	}

	if time.Now().UTC().Before(order.StorageUntil) {
		l.Warn("Precondition failed: storage period not expired", zap.Time("storage_until", order.StorageUntil))
		metrics.OperationErrorsTotal.WithLabelValues("delete_order").Inc()
		return nil, status.Error(codes.FailedPrecondition, "Cannot return: storage period not expired")
	}

	err = s.storage.DeleteOrder(ctx, req.OrderId)
	if err != nil {
		l.Error("Failed to delete order", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("delete_order").Inc()
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "order not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete order: %v", err)
	}

	l.Info("Order deleted (returned to courier) successfully")
	return &emptypb.Empty{}, nil
}

func (s *Server) ListUserOrders(ctx context.Context, req *pb.ListUserOrdersRequest) (*pb.ListUserOrdersResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "ListUserOrders"), zap.String("user_id", req.UserId), zap.Int32("last_n", req.LastN), zap.Bool("active_only", req.ActiveOnly))
	l.Debug("RPC call received")

	if req.UserId == "" {
		l.Warn("Invalid argument: missing user_id")
		metrics.OperationErrorsTotal.WithLabelValues("list_user_orders").Inc()
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	orders, err := s.storage.GetUserOrders(ctx, req.UserId, int(req.LastN), req.ActiveOnly)
	if err != nil {
		l.Error("Failed to list user orders", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("list_user_orders").Inc()
		return nil, status.Errorf(codes.Internal, "failed to list user orders: %v", err)
	}

	protoOrders := make([]*pb.Order, 0, len(orders))
	for _, order := range orders {
		protoOrders = append(protoOrders, toProtoOrder(&order))
	}

	l.Info("User orders listed successfully", zap.Int("count", len(protoOrders)))
	return &pb.ListUserOrdersResponse{Orders: protoOrders}, nil
}

func (s *Server) AddReturn(ctx context.Context, req *pb.AddReturnRequest) (*emptypb.Empty, error) {
	l := s.logger.With(zap.String("rpc_method", "AddReturn"), zap.String("order_id", req.OrderId), zap.String("user_id", req.UserId))
	l.Debug("RPC call received")

	if req.OrderId == "" || req.UserId == "" {
		l.Warn("Invalid argument: missing order_id or user_id")
		metrics.OperationErrorsTotal.WithLabelValues("add_return").Inc()
		return nil, status.Error(codes.InvalidArgument, "order_id and user_id are required")
	}

	ret := storage.Return{
		OrderID:    req.OrderId,
		UserID:     req.UserId,
		ReturnedAt: time.Now().UTC(),
	}

	if err := s.storage.AddReturn(ctx, ret); err != nil {
		l.Error("Failed to add return", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("add_return").Inc()

		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not belong") ||
			strings.Contains(err.Error(), "not in 'issued' status") ||
			strings.Contains(err.Error(), "return period expired") {
			return nil, status.Errorf(codes.FailedPrecondition, "failed to add return: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to process return: %v", err)
	}

	metrics.ReturnsAcceptedTotal.Inc()
	l.Info("Return accepted successfully")
	return &emptypb.Empty{}, nil
}

func (s *Server) ListReturns(ctx context.Context, req *pb.ListReturnsRequest) (*pb.ListReturnsResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "ListReturns"), zap.Int32("page", req.Page), zap.Int32("limit", req.Limit))
	l.Debug("RPC call received")

	page := int(req.Page)
	limit := int(req.Limit)
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	returns, err := s.storage.GetReturns(ctx, page, limit)
	if err != nil {
		l.Error("Failed to list returns", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("list_returns").Inc()
		return nil, status.Errorf(codes.Internal, "failed to list returns: %v", err)
	}

	protoReturns := make([]*pb.Return, 0, len(returns))
	for _, ret := range returns {
		protoReturns = append(protoReturns, toProtoReturn(&ret))
	}

	l.Info("Returns listed successfully", zap.Int("count", len(protoReturns)))
	return &pb.ListReturnsResponse{Returns: protoReturns}, nil
}

func (s *Server) IssueOrders(ctx context.Context, req *pb.IssueOrdersRequest) (*pb.IssueOrdersResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "IssueOrders"), zap.String("user_id", req.UserId), zap.Int("order_count_req", len(req.OrderIds)))
	l.Debug("RPC call received")

	if req.UserId == "" || len(req.OrderIds) == 0 {
		l.Warn("Invalid argument: missing user_id or order_ids")
		metrics.OperationErrorsTotal.WithLabelValues("issue_orders").Inc()
		return nil, status.Error(codes.InvalidArgument, "user_id and at least one order_id are required")
	}

	results, err := s.storage.IssueOrders(ctx, req.UserId, req.OrderIds)

	successCount := 0
	failCount := 0
	protoResults := make([]*pb.IssueOrdersResult, 0, len(results))
	for _, res := range results {
		protoResults = append(protoResults, &pb.IssueOrdersResult{
			OrderId: res.OrderID,
			Success: res.Success,
			Error:   res.Error,
		})
		if res.Success {
			successCount++
			metrics.OrdersIssuedTotal.Inc()
		} else {
			failCount++
			metrics.OperationErrorsTotal.WithLabelValues("issue_orders").Inc()
			l.Warn("Failed to issue specific order", zap.String("order_id", res.OrderID), zap.String("error", res.Error))
		}
	}

	if err != nil {
		l.Error("Transaction error during IssueOrders", zap.Int("success_count", successCount), zap.Int("fail_count", failCount), zap.Error(err))
		return &pb.IssueOrdersResponse{Results: protoResults}, status.Errorf(codes.Internal, "error processing bulk issue: %v", err)
	}

	l.Info("IssueOrders processed", zap.Int("success_count", successCount), zap.Int("fail_count", failCount))
	return &pb.IssueOrdersResponse{Results: protoResults}, nil
}

func (s *Server) AcceptReturns(ctx context.Context, req *pb.AcceptReturnsRequest) (*pb.AcceptReturnsResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "AcceptReturns"), zap.String("user_id", req.UserId), zap.Int("order_count_req", len(req.OrderIds)))
	l.Debug("RPC call received")

	if req.UserId == "" || len(req.OrderIds) == 0 {
		l.Warn("Invalid argument: missing user_id or order_ids")
		metrics.OperationErrorsTotal.WithLabelValues("accept_returns").Inc()
		return nil, status.Error(codes.InvalidArgument, "user_id and at least one order_id are required")
	}

	results, err := s.storage.AcceptReturns(ctx, req.UserId, req.OrderIds)

	successCount := 0
	failCount := 0
	protoResults := make([]*pb.AcceptReturnsResult, 0, len(results))
	for _, res := range results {
		protoResults = append(protoResults, &pb.AcceptReturnsResult{
			OrderId: res.OrderID,
			Success: res.Success,
			Error:   res.Error,
		})
		if res.Success {
			successCount++
			metrics.ReturnsAcceptedTotal.Inc()
		} else {
			failCount++
			metrics.OperationErrorsTotal.WithLabelValues("accept_returns").Inc()
			l.Warn("Failed to accept specific return", zap.String("order_id", res.OrderID), zap.String("error", res.Error))
		}
	}

	if err != nil {
		l.Error("Transaction error during AcceptReturns", zap.Int("success_count", successCount), zap.Int("fail_count", failCount), zap.Error(err))
		return &pb.AcceptReturnsResponse{Results: protoResults}, status.Errorf(codes.Internal, "error processing bulk return acceptance: %v", err)
	}

	l.Info("AcceptReturns processed", zap.Int("success_count", successCount), zap.Int("fail_count", failCount))
	return &pb.AcceptReturnsResponse{Results: protoResults}, nil
}

func (s *Server) GetOrderHistory(ctx context.Context, req *pb.GetOrderHistoryRequest) (*pb.GetOrderHistoryResponse, error) {
	l := s.logger.With(zap.String("rpc_method", "GetOrderHistory"), zap.String("order_id", req.OrderId))
	l.Debug("RPC call received")

	if req.OrderId == "" {
		l.Warn("Invalid argument: missing order_id")
		metrics.OperationErrorsTotal.WithLabelValues("get_order_history").Inc()
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	history, err := s.storage.GetOrderHistory(ctx, req.OrderId)
	if err != nil {
		l.Error("Failed to get order history", zap.Error(err))
		metrics.OperationErrorsTotal.WithLabelValues("get_order_history").Inc()
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "history not found for order: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get order history: %v", err)
	}

	protoHistory := make([]*pb.HistoryEntry, 0, len(history))
	for _, entry := range history {
		protoHistory = append(protoHistory, toProtoHistoryEntry(&entry))
	}

	l.Info("Order history retrieved successfully", zap.Int("count", len(protoHistory)))
	return &pb.GetOrderHistoryResponse{History: protoHistory}, nil
}
