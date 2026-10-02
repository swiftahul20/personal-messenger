package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swiftahul20/personal-messenger/internal/api"
	"github.com/swiftahul20/personal-messenger/internal/chat"
	"github.com/swiftahul20/personal-messenger/internal/data"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := env("DATABASE_URL", "postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable")
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("configure database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	store := data.NewPostgres(pool)
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("run database schema: %v", err)
	}

	hub := chat.NewHub(store)
	go hub.Run(ctx)
	address := env("ADDR", ":8080")
	server := &http.Server{
		Addr: address, Handler: api.NewHandler(store, store, store, hub).Routes(),
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
	}()

	log.Printf("chat server listening on %s", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http server: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
