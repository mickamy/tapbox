package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mickamy/tapbox/example/internal/db"
	"github.com/mickamy/tapbox/example/internal/env"
)

type searchResult struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func main() {
	dbDSN := env.Or("DB_DSN", "postgres://tapbox:tapbox@localhost:5433/tapbox_example?sslmode=disable")
	addr := env.Or("HTTP_ADDR", ":4000")

	log.Printf("HTTP backend starting (DB: %s)", dbDSN)

	ctx := context.Background()
	pool, err := db.ConnectWithRetry(ctx, dbDSN, 30)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /search", handleSearch(pool))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("HTTP backend listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func handleSearch(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "q parameter is required"})
			return
		}

		rows, err := pool.Query(r.Context(),
			"SELECT id, title, body, created_at FROM notes WHERE title ILIKE '%' || $1 || '%' ORDER BY created_at DESC LIMIT 20",
			q,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		var results []searchResult
		for rows.Next() {
			var r searchResult
			var createdAt time.Time
			if err := rows.Scan(&r.ID, &r.Title, &r.Body, &createdAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			r.CreatedAt = createdAt.Format(time.RFC3339)
			results = append(results, r)
		}

		writeJSON(w, http.StatusOK, results)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}

