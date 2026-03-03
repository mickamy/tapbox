package ui

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mickamy/tapbox/internal/trace"
	"github.com/mickamy/tapbox/web"
)

var (
	pgxPoolOnce sync.Once
	pgxPoolInst *pgxpool.Pool
	errPGXPool  error
)

func getPGXPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pgxPoolOnce.Do(func() {
		pgxPoolInst, errPGXPool = pgxpool.New(ctx, dsn)
	})
	if errPGXPool != nil {
		return nil, fmt.Errorf("creating pgx pool: %w", errPGXPool)
	}
	return pgxPoolInst, nil
}

type Server struct {
	handler http.Handler
	hub     *Hub
}

func NewServer(store trace.Store, explainDSN string) *Server {
	api := NewAPI(store, explainDSN)
	hub := NewHub(store)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/traces", api.HandleListTraces)
	mux.HandleFunc("GET /api/traces/{traceID}", api.HandleGetTrace)
	mux.HandleFunc("GET /api/spans/{traceID}/{spanID}", api.HandleGetSpan)
	mux.HandleFunc("POST /api/explain", api.HandleExplain)
	mux.HandleFunc("GET /events", hub.HandleSSE)

	// Static files
	staticFS, _ := fs.Sub(web.Static, ".")
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

	return &Server{handler: mux, hub: hub}
}

func (s *Server) Start(ctx context.Context) {
	s.hub.Start(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.handler
}
