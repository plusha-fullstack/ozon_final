package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	pb "gitlab.ozon.dev/pupkingeorgij/homework/internal/api"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/cache"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/grpcserver"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/kafka"
	applogger "gitlab.ozon.dev/pupkingeorgij/homework/internal/logger"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository/postgresql"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/server"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

const (
	httpPort = "9000"
	grpcPort = "8001"
)

func main() {
	logger := applogger.New()
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("Application starting...")

	dbPool, err := db.NewDb(ctx)
	if err != nil {
		logger.Fatal("Database init error", zap.Error(err))
	}
	logger.Info("Database connection established")

	db.InitAdmin(dbPool)

	orderRepo := postgresql.NewOrderRepo(dbPool)
	returnRepo := postgresql.NewReturnRepo(dbPool)
	historyRepo := postgresql.NewHistoryRepo(dbPool)
	userRepo := postgresql.NewUserRepo(dbPool)
	outboxRepo := postgresql.NewOutboxTaskRepo()

	orderCacheRepo, ok := orderRepo.(cache.OrderRepository)
	if !ok {
		logger.Fatal("Order repository does not implement cache.OrderRepository interface")
	}
	orderCache := cache.NewOrderCache(orderCacheRepo)
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()
	if err := orderCache.LoadInitialData(initCtx); err != nil {
		logger.Warn("Failed to load initial order cache data, starting empty", zap.Error(err))
	}

	stg := storage.NewStorage(ctx, dbPool, orderRepo, returnRepo, historyRepo, userRepo, outboxRepo, orderCache)

	kafkaProducer := kafka.NewConsoleProducer()
	publisherConfig := kafka.PublisherConfig{
		PollInterval: 5 * time.Second,
		BatchSize:    10,
		MaxAttempts:  5,
	}
	outboxPublisher := kafka.NewPublisher(dbPool, outboxRepo, kafkaProducer, publisherConfig)

	restSrv := server.New(stg, userRepo)
	grpcImpl := grpcserver.NewServer(*stg, logger.Named("grpc_server"))

	grpcSrv := grpc.NewServer()
	pb.RegisterOrderServiceServer(grpcSrv, grpcImpl)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("Starting Kafka Publisher...")
		outboxPublisher.Run(gCtx)
		logger.Info("Kafka Publisher stopped.")
		return nil
	})

	g.Go(func() error {
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			logger.Error("Failed to listen on gRPC port", zap.String("port", grpcPort), zap.Error(err))
			return fmt.Errorf("failed to listen on gRPC port %s: %w", grpcPort, err)
		}
		logger.Info("gRPC Server starting", zap.String("port", grpcPort))
		go func() {
			<-gCtx.Done()
			logger.Info("Shutting down gRPC server...")
			grpcSrv.GracefulStop()
			logger.Info("gRPC server stopped.")
		}()
		if err := grpcSrv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error("gRPC server failed", zap.Error(err))
			return fmt.Errorf("gRPC server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		logger.Info("HTTP Server starting", zap.String("port", httpPort))
		go func() {
			<-gCtx.Done()
			logger.Info("Shutting down HTTP server...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := restSrv.Shutdown(shutdownCtx); err != nil {
				logger.Error("HTTP server shutdown error", zap.Error(err))
			}
			logger.Info("HTTP server stopped.")
		}()
		if err := restSrv.Run(gCtx, httpPort); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", zap.Error(err))
			return fmt.Errorf("HTTP server failed: %w", err)
		}
		return nil
	})

	logger.Info("Application started. Press Ctrl+C to exit.")

	if err := g.Wait(); err != nil {
		logger.Error("Server group encountered an error", zap.Error(err))
	}

	logger.Info("Shutting down Kafka Publisher (explicitly)...")
	outboxPublisher.Shutdown()

	logger.Info("Application gracefully stopped.")
}
