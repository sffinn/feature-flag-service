package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	rediscache "github.com/featureflag-service/featureflag-service/internal/cache/redis"
	"github.com/featureflag-service/featureflag-service/internal/flag"
	"github.com/featureflag-service/featureflag-service/internal/httpapi"
	"github.com/featureflag-service/featureflag-service/internal/migrate"
	"github.com/featureflag-service/featureflag-service/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	if err := migrate.Up(ctx, pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	store := postgres.NewStore(pool)
	cache := rediscache.New(rdb)
	svc := flag.NewService(store, cache, cfg.CacheTTL)
	api := httpapi.NewServer(svc, logger)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("shutting down")
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

type config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	CacheTTL    time.Duration
}

func loadConfig() (config, error) {
	cfg := config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		CacheTTL:    60 * time.Second,
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return cfg, fmt.Errorf("REDIS_URL is required")
	}
	if v := os.Getenv("CACHE_TTL_SECONDS"); v != "" {
		sec, err := strconv.Atoi(v)
		if err != nil || sec <= 0 {
			return cfg, fmt.Errorf("invalid CACHE_TTL_SECONDS: %q", v)
		}
		cfg.CacheTTL = time.Duration(sec) * time.Second
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
