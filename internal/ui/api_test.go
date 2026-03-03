package ui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mickamy/tapbox/internal/trace"
	"github.com/mickamy/tapbox/internal/ui"
)

func seedStore(store *trace.MemStore) {
	now := time.Now()
	store.Add(&trace.Span{
		TraceID:        "aaaa1111bbbb2222cccc3333dddd4444",
		SpanID:         "1111222233334444",
		Kind:           trace.SpanHTTP,
		Name:           "GET /users",
		Start:          now,
		Duration:       50,
		Status:         trace.StatusOK,
		HTTPMethod:     "GET",
		HTTPPath:       "/users",
		HTTPStatusCode: 200,
	})
	store.Add(&trace.Span{
		TraceID:     "aaaa1111bbbb2222cccc3333dddd4444",
		SpanID:      "5555666677778888",
		ParentID:    "1111222233334444",
		Kind:        trace.SpanSQL,
		Name:        "SELECT id, name FROM users",
		Start:       now.Add(10 * time.Millisecond),
		Duration:    20,
		Status:      trace.StatusOK,
		SQLQuery:    "SELECT id, name FROM users",
		SQLRowCount: 5,
	})
}

func TestAPI_HandleListTraces(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	seedStore(store)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/traces?limit=50", nil)
	rec := httptest.NewRecorder()
	api.HandleListTraces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var traces []trace.Trace
	if err := json.NewDecoder(rec.Body).Decode(&traces); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(traces) != 1 {
		t.Errorf("len(traces) = %d, want 1", len(traces))
	}
}

func TestAPI_HandleListTraces_Empty(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/traces", nil)
	rec := httptest.NewRecorder()
	api.HandleListTraces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPI_HandleListTraces_DefaultLimit(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	seedStore(store)
	api := ui.NewAPI(store, "")

	// No limit param → should use default (50)
	req := httptest.NewRequest(http.MethodGet, "/api/traces", nil)
	rec := httptest.NewRecorder()
	api.HandleListTraces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPI_HandleGetTrace(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	seedStore(store)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/traces/aaaa1111bbbb2222cccc3333dddd4444", nil)
	rec := httptest.NewRecorder()
	api.HandleGetTrace(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var tr trace.Trace
	if err := json.NewDecoder(rec.Body).Decode(&tr); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if tr.TraceID != "aaaa1111bbbb2222cccc3333dddd4444" {
		t.Errorf("TraceID = %q, want expected ID", tr.TraceID)
	}
	if len(tr.Spans) != 2 {
		t.Errorf("len(Spans) = %d, want 2", len(tr.Spans))
	}
}

func TestAPI_HandleGetTrace_NotFound(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/traces/nonexistent", nil)
	rec := httptest.NewRecorder()
	api.HandleGetTrace(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPI_HandleGetSpan(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	seedStore(store)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/spans/aaaa1111bbbb2222cccc3333dddd4444/5555666677778888", nil)
	rec := httptest.NewRecorder()
	api.HandleGetSpan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var span trace.Span
	if err := json.NewDecoder(rec.Body).Decode(&span); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if span.SpanID != "5555666677778888" {
		t.Errorf("SpanID = %q, want %q", span.SpanID, "5555666677778888")
	}
	if span.SQLQuery != "SELECT id, name FROM users" {
		t.Errorf("SQLQuery = %q, want %q", span.SQLQuery, "SELECT id, name FROM users")
	}
}

func TestAPI_HandleGetSpan_NotFound(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	seedStore(store)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/spans/aaaa1111bbbb2222cccc3333dddd4444/nonexistent", nil)
	rec := httptest.NewRecorder()
	api.HandleGetSpan(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPI_HandleGetSpan_TraceNotFound(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/spans/nonexistent/nonexistent", nil)
	rec := httptest.NewRecorder()
	api.HandleGetSpan(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPI_HandleGetSpan_MissingParams(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	api := ui.NewAPI(store, "")

	req := httptest.NewRequest(http.MethodGet, "/api/spans/onlyoneid", nil)
	rec := httptest.NewRecorder()
	api.HandleGetSpan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPI_HandleExplain_NoDSN(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	api := ui.NewAPI(store, "") // empty DSN

	req := httptest.NewRequest(http.MethodPost, "/api/explain", nil)
	rec := httptest.NewRecorder()
	api.HandleExplain(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
