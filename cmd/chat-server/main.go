package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swiftahul20/personal-messenger/internal/api"
	"github.com/swiftahul20/personal-messenger/internal/bot"
	"github.com/swiftahul20/personal-messenger/internal/chat"
	"github.com/swiftahul20/personal-messenger/internal/data"
	"github.com/swiftahul20/personal-messenger/internal/user"
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

	botUsername := strings.TrimSpace(env("BOT_USERNAME", user.DefaultBotUsername))
	if botUsername == "" {
		botUsername = user.DefaultBotUsername
	}
	botUser, err := store.EnsureBotUser(ctx, botUsername)
	if err != nil {
		log.Fatalf("create bot user %q: %v", botUsername, err)
	}
	if err := store.EnsureBotBuddyForAllUsers(ctx, botUser.ID); err != nil {
		log.Fatalf("link bot to all users: %v", err)
	}
	log.Printf("bot user ready: %s (%d)", botUser.Username, botUser.ID)

	hub := chat.NewHub(store)
	hub.SetBotUserID(botUser.ID)
	go hub.Run(ctx)

	botEnabled := envBool("BOT_ENABLED", true)
	botRPM := envInt("BOT_USER_RPM", 5)
	botDailyCap := envInt("BOT_DAILY_CAP", 100)
	var responder bot.Responder = bot.StaticResponder{}
	if botEnabled {
		apiKey := strings.TrimSpace(env("GEMINI_API_KEY", ""))
		if apiKey == "" {
			log.Printf("GEMINI_API_KEY is not set; falling back to static bot replies")
		} else {
			responder = bot.NewGeminiResponder(apiKey, env("GEMINI_MODEL", bot.DefaultGeminiModel))
			if responder == nil {
				log.Printf("failed to initialize Gemini responder; using static bot replies")
				responder = bot.StaticResponder{}
			}
		}
	}
	botRunner := bot.NewBot(hub, store, responder, botEnabled, botUser.ID, botRPM, botDailyCap)
	if botEnabled {
		go botRunner.Run(ctx)
		log.Printf("bot enabled: %s (%d)", botUser.Username, botUser.ID)
	} else {
		log.Printf("bot disabled via BOT_ENABLED")
	}

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

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("invalid integer for %s=%q; using %d", key, value, fallback)
		return fallback
	}
	return n
}
