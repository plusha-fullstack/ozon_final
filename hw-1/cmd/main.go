package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/cache"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/repository/postgresql"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/server"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dbPool, err := db.NewDb(ctx)
	if err != nil {
		fmt.Println("Database init error:", err)
		return
	}

	db.InitAdmin(dbPool)

	var orderRepo storage.OrderRepository = postgresql.NewOrderRepo(dbPool)
	var returnRepo storage.ReturnRepository = postgresql.NewReturnRepo(dbPool)
	var historyRepo storage.HistoryRepository = postgresql.NewHistoryRepo(dbPool)
	var userRepo storage.UserRepository = postgresql.NewUserRepo(dbPool)

	orderCacheRepo, ok := orderRepo.(cache.OrderRepository)
	if !ok {
		log.Fatal("Order repository does not implement cache.OrderRepository interface")
		return
	}
	orderCache := cache.NewOrderCache(orderCacheRepo)

	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()
	if err := orderCache.LoadInitialData(initCtx); err != nil {
		log.Printf("WARNING: Failed to load initial order cache data: %v. Starting with an empty cache.", err)
	}

	stg := storage.NewStorage(ctx, dbPool, orderRepo, returnRepo, historyRepo, userRepo, orderCache)

	srv := server.New(stg, userRepo)

	go func() {
		if err := srv.Run(ctx, "9000"); err != nil {
			if err != http.ErrServerClosed {
				log.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	log.Println("Server started on port 9000")

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server gracefully stopped")
}
