package trace_test

import (
	"testing"

	"github.com/mickamy/tapbox/internal/trace"
)

func TestNewTraceID_Length(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	id := trace.NewTraceID()
	if len(id) != 32 {
		t.Errorf("TraceID length = %d, want 32", len(id))
	}
}

func TestNewTraceID_Uniqueness(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	seen := make(map[string]struct{})
	for range 100 {
		id := trace.NewTraceID()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate TraceID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewTraceID_HexEncoded(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	id := trace.NewTraceID()
	for _, c := range id {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			t.Errorf("TraceID contains non-hex char: %c in %s", c, id)
		}
	}
}

func TestNewSpanID_Length(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	id := trace.NewSpanID()
	if len(id) != 16 {
		t.Errorf("SpanID length = %d, want 16", len(id))
	}
}

func TestNewSpanID_Uniqueness(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	seen := make(map[string]struct{})
	for range 100 {
		id := trace.NewSpanID()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate SpanID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestSpanKind_String(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	tests := []struct {
		kind trace.SpanKind
		want string
	}{
		{trace.SpanHTTP, "http"},
		{trace.SpanConnect, "connect"},
		{trace.SpanGRPC, "grpc"},
		{trace.SpanSQL, "sql"},
		{trace.SpanKind(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("SpanKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestSpanStatus_String(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	if got := trace.StatusOK.String(); got != "ok" {
		t.Errorf("StatusOK.String() = %q, want %q", got, "ok")
	}
	if got := trace.StatusError.String(); got != "error" {
		t.Errorf("StatusError.String() = %q, want %q", got, "error")
	}
}

func TestTrace_RootSpan_FindsParentless(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	tr := &trace.Trace{
		Spans: []*trace.Span{
			{SpanID: "child", ParentID: "root"},
			{SpanID: "root", ParentID: ""},
		},
	}
	root := tr.RootSpan()
	if root == nil {
		t.Fatal("RootSpan returned nil")
	}
	if root.SpanID != "root" {
		t.Errorf("RootSpan().SpanID = %q, want %q", root.SpanID, "root")
	}
}

func TestTrace_RootSpan_FallsBackToFirst(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	tr := &trace.Trace{
		Spans: []*trace.Span{
			{SpanID: "a", ParentID: "external"},
			{SpanID: "b", ParentID: "external"},
		},
	}
	root := tr.RootSpan()
	if root == nil {
		t.Fatal("RootSpan returned nil")
	}
	if root.SpanID != "a" {
		t.Errorf("RootSpan().SpanID = %q, want %q (first span)", root.SpanID, "a")
	}
}

func TestTrace_RootSpan_Empty(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	tr := &trace.Trace{}
	if tr.RootSpan() != nil {
		t.Error("RootSpan should return nil for empty trace")
	}
}
