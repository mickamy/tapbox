package ui_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mickamy/tapbox/internal/trace"
	"github.com/mickamy/tapbox/internal/ui"
)

func TestServer_Handler_ServesStaticFiles(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	store := trace.NewMemStore(100)
	srv := ui.NewServer(store, "")
	srv.Start(ctx)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("expected Content-Type header")
	}
}

func TestServer_Handler_ServesAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	store := trace.NewMemStore(100)
	srv := ui.NewServer(store, "")
	srv.Start(ctx)

	req := httptest.NewRequest(http.MethodGet, "/api/traces", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestServer_Handler_RootServesIndex(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	store := trace.NewMemStore(100)
	srv := ui.NewServer(store, "")
	srv.Start(ctx)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
