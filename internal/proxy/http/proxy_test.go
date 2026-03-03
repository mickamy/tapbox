package http_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpproxy "github.com/mickamy/tapbox/internal/proxy/http"
	"github.com/mickamy/tapbox/internal/trace"
)

func TestProxy_ForwardsRequest(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", body, `{"status":"ok"}`)
	}
}

func TestProxy_CreatesSpan(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	tr := traces[0]
	if len(tr.Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(tr.Spans))
	}
	span := tr.Spans[0]
	if span.Kind != trace.SpanHTTP {
		t.Errorf("Kind = %v, want SpanHTTP", span.Kind)
	}
	if span.HTTPMethod != http.MethodPost {
		t.Errorf("HTTPMethod = %q, want %q", span.HTTPMethod, http.MethodPost)
	}
	if span.HTTPPath != "/users" {
		t.Errorf("HTTPPath = %q, want %q", span.HTTPPath, "/users")
	}
	if span.HTTPStatusCode != http.StatusOK {
		t.Errorf("HTTPStatusCode = %d, want %d", span.HTTPStatusCode, http.StatusOK)
	}
	if span.HTTPRequestBody != `{"name":"test"}` {
		t.Errorf("HTTPRequestBody = %q, want request body", span.HTTPRequestBody)
	}
}

func TestProxy_InjectsTraceparent(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	var receivedHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("Traceparent")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if receivedHeader == "" {
		t.Fatal("upstream did not receive Traceparent header")
	}
	// Format: 00-{traceID(32)}-{spanID(16)}-01
	parts := strings.Split(receivedHeader, "-")
	if len(parts) != 4 {
		t.Fatalf("Traceparent has %d parts, want 4", len(parts))
	}
	if parts[0] != "00" {
		t.Errorf("version = %q, want %q", parts[0], "00")
	}
	if len(parts[1]) != 32 {
		t.Errorf("traceID length = %d, want 32", len(parts[1]))
	}
	if len(parts[2]) != 16 {
		t.Errorf("spanID length = %d, want 16", len(parts[2]))
	}
	if parts[3] != "01" {
		t.Errorf("flags = %q, want %q", parts[3], "01")
	}
}

func TestProxy_PropagatesExistingTraceparent(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	var receivedHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("Traceparent")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	existingTraceID := "abcdef1234567890abcdef1234567890"
	existingSpanID := "1234567890abcdef"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Traceparent", "00-"+existingTraceID+"-"+existingSpanID+"-01")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	// Upstream should receive the same trace ID but a new span ID
	parts := strings.Split(receivedHeader, "-")
	if len(parts) != 4 {
		t.Fatalf("Traceparent has %d parts, want 4", len(parts))
	}
	if parts[1] != existingTraceID {
		t.Errorf("traceID = %q, want %q (should propagate)", parts[1], existingTraceID)
	}
	if parts[2] == existingSpanID {
		t.Error("spanID should be different (new child span)")
	}

	// The recorded span should have the original span as parent
	traces := store.ListTraces(0, 10)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	span := traces[0].Spans[0]
	if span.TraceID != existingTraceID {
		t.Errorf("span TraceID = %q, want %q", span.TraceID, existingTraceID)
	}
	if span.ParentID != existingSpanID {
		t.Errorf("span ParentID = %q, want %q", span.ParentID, existingSpanID)
	}
}

func TestProxy_ErrorStatus(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	if len(traces) != 1 || len(traces[0].Spans) != 1 {
		t.Fatal("expected 1 trace with 1 span")
	}
	span := traces[0].Spans[0]
	if span.Status != trace.StatusError {
		t.Errorf("Status = %v, want StatusError for 500 response", span.Status)
	}
	if span.HTTPStatusCode != http.StatusInternalServerError {
		t.Errorf("HTTPStatusCode = %d, want %d", span.HTTPStatusCode, http.StatusInternalServerError)
	}
}

func TestProxy_BodyCaptureLimited(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and echo back to confirm request was received
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	maxBody := 10
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, maxBody)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	longBody := strings.Repeat("x", 100)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(longBody))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	if len(traces) != 1 || len(traces[0].Spans) != 1 {
		t.Fatal("expected 1 trace with 1 span")
	}
	span := traces[0].Spans[0]
	// Request body should be limited by maxBodySize (captureBody reads at most maxBody bytes)
	if len(span.HTTPRequestBody) > maxBody {
		t.Errorf("HTTPRequestBody length = %d, want <= %d", len(span.HTTPRequestBody), maxBody)
	}
}

func TestProxy_SpanDuration(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	span := traces[0].Spans[0]
	if span.Duration <= 0 {
		t.Errorf("Duration = %f, want > 0", span.Duration)
	}
}

func TestProxy_ConnectSpan(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"greeting":"hello"}`))
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	body := strings.NewReader(`{"name":"world"}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/acme.greeter.v1.GreeterService/Greet",
		body,
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	span := traces[0].Spans[0]
	if span.Kind != trace.SpanConnect {
		t.Errorf("Kind = %v, want SpanConnect", span.Kind)
	}
	if span.ConnectService != "acme.greeter.v1.GreeterService" {
		t.Errorf("ConnectService = %q, want %q", span.ConnectService, "acme.greeter.v1.GreeterService")
	}
	if span.ConnectMethod != "Greet" {
		t.Errorf("ConnectMethod = %q, want %q", span.ConnectMethod, "Greet")
	}
	if span.ConnectHTTPStatus != http.StatusOK {
		t.Errorf("ConnectHTTPStatus = %d, want %d", span.ConnectHTTPStatus, http.StatusOK)
	}
	if span.ConnectRequestBody != `{"name":"world"}` {
		t.Errorf("ConnectRequestBody = %q, want request body", span.ConnectRequestBody)
	}
}

func TestProxy_ConnectError(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","message":"user not found"}`))
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/acme.user.v1.UserService/GetUser", strings.NewReader(`{"id":"123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	span := traces[0].Spans[0]
	if span.Kind != trace.SpanConnect {
		t.Errorf("Kind = %v, want SpanConnect", span.Kind)
	}
	if span.Status != trace.StatusError {
		t.Errorf("Status = %v, want StatusError", span.Status)
	}
	if span.ConnectErrorCode != "not_found" {
		t.Errorf("ConnectErrorCode = %q, want %q", span.ConnectErrorCode, "not_found")
	}
	if span.ConnectErrorMessage != "user not found" {
		t.Errorf("ConnectErrorMessage = %q, want %q", span.ConnectErrorMessage, "user not found")
	}
}

func TestProxy_RegularHTTPPostNotConnect(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer upstream.Close()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)
	proxy, err := httpproxy.NewProxy(upstream.URL, collector, 64*1024)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	traces := store.ListTraces(0, 10)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	span := traces[0].Spans[0]
	if span.Kind != trace.SpanHTTP {
		t.Errorf("Kind = %v, want SpanHTTP (regular REST POST should not be Connect)", span.Kind)
	}
}
