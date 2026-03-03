package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/mickamy/tapbox/internal/trace"
)

type API struct {
	store      trace.Store
	explainDSN string
}

func NewAPI(store trace.Store, explainDSN string) *API {
	return &API{store: store, explainDSN: explainDSN}
}

func (a *API) HandleListTraces(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	traces := a.store.ListTraces(offset, limit)
	writeJSON(w, traces)
}

func (a *API) HandleGetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("traceID")
	if traceID == "" {
		http.Error(w, "trace ID required", http.StatusBadRequest)
		return
	}

	t := a.store.GetTrace(traceID)
	if t == nil {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	writeJSON(w, t)
}

func (a *API) HandleGetSpan(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("traceID")
	spanID := r.PathValue("spanID")
	if traceID == "" || spanID == "" {
		http.Error(w, "trace ID and span ID required", http.StatusBadRequest)
		return
	}

	t := a.store.GetTrace(traceID)
	if t == nil {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	for _, s := range t.Spans {
		if s.SpanID == spanID {
			writeJSON(w, s)
			return
		}
	}

	http.Error(w, "span not found", http.StatusNotFound)
}

type explainRequest struct {
	Query   string `json:"query"`
	Analyze bool   `json:"analyze"`
}

type explainResult struct {
	Plan string `json:"plan"`
}

func (a *API) HandleExplain(w http.ResponseWriter, r *http.Request) {
	if a.explainDSN == "" {
		writeJSONError(w, http.StatusBadRequest, "explain not configured (no --sql-target or --explain-dsn)")
		return
	}

	var req explainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	plan, err := runExplain(r.Context(), a.explainDSN, req.Query, req.Analyze)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "explain failed: "+err.Error())
		return
	}

	writeJSON(w, explainResult{Plan: plan})
}

func runExplain(ctx context.Context, dsn, query string, analyze bool) (string, error) {
	pgxPool, err := getPGXPool(ctx, dsn)
	if err != nil {
		return "", err
	}

	// Run EXPLAIN inside a read-only transaction to prevent data modification.
	// This guards against malicious input such as "DELETE FROM users" being
	// executed via EXPLAIN ANALYZE, which actually runs the query.
	tx, err := pgxPool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		return "", fmt.Errorf("setting read-only transaction: %w", err)
	}

	prefix := "EXPLAIN "
	if analyze {
		prefix = "EXPLAIN ANALYZE "
	}
	rows, err := tx.Query(ctx, prefix+query)
	if err != nil {
		return "", fmt.Errorf("executing explain: %w", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", fmt.Errorf("scanning explain row: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterating explain rows: %w", err)
	}
	return strings.Join(lines, "\n"), nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("json encode error: %v", err)
	}
}
