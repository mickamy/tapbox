package trace_test

import (
	"testing"

	"github.com/mickamy/tapbox/internal/trace"
)

func TestCollector_Submit(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)

	span := newSpan("trace1", "span1")
	span.Kind = trace.SpanHTTP
	span.Name = "GET /test"

	collector.Submit(span)

	tr := store.GetTrace("trace1")
	if tr == nil {
		t.Fatal("expected trace to be stored after Submit")
	}
	if len(tr.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(tr.Spans))
	}
	if tr.Spans[0].Name != "GET /test" {
		t.Errorf("Span.Name = %q, want %q", tr.Spans[0].Name, "GET /test")
	}
}

func TestCollector_SubmitMultiple(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	collector := trace.NewCollector(store)

	collector.Submit(newSpan("trace1", "span1"))
	collector.Submit(newSpan("trace1", "span2"))
	collector.Submit(newSpan("trace2", "span3"))

	t1 := store.GetTrace("trace1")
	if t1 == nil || len(t1.Spans) != 2 {
		t.Errorf("trace1 should have 2 spans, got %v", t1)
	}

	t2 := store.GetTrace("trace2")
	if t2 == nil || len(t2.Spans) != 1 {
		t.Errorf("trace2 should have 1 span, got %v", t2)
	}
}
