package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectWithRetry creates a pgxpool.Pool with retry logic.
// The pool is configured with a long health check period to avoid noisy
// "-- ping" queries in traces.
func ConnectWithRetry(ctx context.Context, dsn string, maxAttempts int) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing DSN: %w", err)
	}
	config.HealthCheckPeriod = 5 * time.Minute
	config.MinConns = 1

	var lastErr error
	for i := range maxAttempts {
		log.Printf("DB connection attempt %d/%d ...", i+1, maxAttempts)
		connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		pool, err := pgxpool.NewWithConfig(connCtx, config)
		if err != nil {
			cancel()
			lastErr = err
			log.Printf("  pool create failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if err := pool.Ping(connCtx); err != nil {
			cancel()
			lastErr = err
			pool.Close()
			log.Printf("  ping failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		cancel()
		return pool, nil
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}
