package trace_test

import (
	"testing"
	"time"

	"github.com/mickamy/tapbox/internal/trace"
)

func newSpan(traceID, spanID string) *trace.Span {
	return &trace.Span{
		TraceID: traceID,
		SpanID:  spanID,
		Start:   time.Now(),
	}
}

func TestMemStore_AddAndGetTrace(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	span := newSpan("trace1", "span1")
	store.Add(span)

	tr := store.GetTrace("trace1")
	if tr == nil {
		t.Fatal("GetTrace returned nil")
	}
	if tr.TraceID != "trace1" {
		t.Errorf("TraceID = %q, want %q", tr.TraceID, "trace1")
	}
	if len(tr.Spans) != 1 {
		t.Errorf("len(Spans) = %d, want 1", len(tr.Spans))
	}
}

func TestMemStore_GetTrace_NotFound(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	if tr := store.GetTrace("nonexistent"); tr != nil {
		t.Error("expected nil for nonexistent trace")
	}
}

func TestMemStore_MultipleSpansSameTrace(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	store.Add(newSpan("trace1", "span1"))
	store.Add(newSpan("trace1", "span2"))
	store.Add(newSpan("trace1", "span3"))

	tr := store.GetTrace("trace1")
	if tr == nil {
		t.Fatal("GetTrace returned nil")
	}
	if len(tr.Spans) != 3 {
		t.Errorf("len(Spans) = %d, want 3", len(tr.Spans))
	}
}

func TestMemStore_ListTraces_NewestFirst(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)

	for i := range 5 {
		s := newSpan("trace"+string(rune('a'+i)), "span1")
		s.Start = time.Now().Add(time.Duration(i) * time.Second)
		store.Add(s)
	}

	traces := store.ListTraces(0, 10)
	if len(traces) != 5 {
		t.Fatalf("len(traces) = %d, want 5", len(traces))
	}
	// Newest first = last inserted first
	if traces[0].TraceID != "tracee" {
		t.Errorf("first trace = %q, want %q", traces[0].TraceID, "tracee")
	}
	if traces[4].TraceID != "tracea" {
		t.Errorf("last trace = %q, want %q", traces[4].TraceID, "tracea")
	}
}

func TestMemStore_ListTraces_Pagination(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	for i := range 10 {
		store.Add(newSpan("trace"+string(rune('a'+i)), "span1"))
	}

	// Get first page
	page1 := store.ListTraces(0, 3)
	if len(page1) != 3 {
		t.Fatalf("page1 len = %d, want 3", len(page1))
	}

	// Get second page
	page2 := store.ListTraces(3, 3)
	if len(page2) != 3 {
		t.Fatalf("page2 len = %d, want 3", len(page2))
	}

	// Pages should not overlap
	if page1[0].TraceID == page2[0].TraceID {
		t.Error("pages should not overlap")
	}
}

func TestMemStore_ListTraces_OffsetBeyondTotal(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	store.Add(newSpan("trace1", "span1"))

	traces := store.ListTraces(10, 10)
	if len(traces) != 0 {
		t.Errorf("expected empty result for offset beyond total, got %d", len(traces))
	}
}

func TestMemStore_ListTraces_Empty(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	traces := store.ListTraces(0, 10)
	if traces != nil {
		t.Errorf("expected nil for empty store, got %v", traces)
	}
}

func TestMemStore_RingBufferEviction(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(3)

	store.Add(newSpan("trace1", "span1"))
	store.Add(newSpan("trace2", "span1"))
	store.Add(newSpan("trace3", "span1"))

	// All 3 should exist
	for _, id := range []string{"trace1", "trace2", "trace3"} {
		if store.GetTrace(id) == nil {
			t.Errorf("trace %q should exist before eviction", id)
		}
	}

	// Adding 4th should evict trace1
	store.Add(newSpan("trace4", "span1"))
	if store.GetTrace("trace1") != nil {
		t.Error("trace1 should have been evicted")
	}
	if store.GetTrace("trace4") == nil {
		t.Error("trace4 should exist")
	}

	// Adding 5th should evict trace2
	store.Add(newSpan("trace5", "span1"))
	if store.GetTrace("trace2") != nil {
		t.Error("trace2 should have been evicted")
	}
	if store.GetTrace("trace5") == nil {
		t.Error("trace5 should exist")
	}
}

func TestMemStore_Subscribe(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	ch := store.Subscribe()
	defer store.Unsubscribe(ch)

	span := newSpan("trace1", "span1")
	store.Add(span)

	select {
	case received := <-ch:
		if received.SpanID != "span1" {
			t.Errorf("received SpanID = %q, want %q", received.SpanID, "span1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscribed span")
	}
}

func TestMemStore_MultipleSubscribers(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	ch1 := store.Subscribe()
	ch2 := store.Subscribe()
	defer store.Unsubscribe(ch1)
	defer store.Unsubscribe(ch2)

	store.Add(newSpan("trace1", "span1"))

	for _, ch := range []<-chan *trace.Span{ch1, ch2} {
		select {
		case received := <-ch:
			if received.SpanID != "span1" {
				t.Errorf("received SpanID = %q, want %q", received.SpanID, "span1")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for subscribed span")
		}
	}
}

func TestMemStore_Unsubscribe(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	ch := store.Subscribe()
	store.Unsubscribe(ch)

	// After unsubscribe, channel should be closed.
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestMemStore_TraceDuration(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	store := trace.NewMemStore(100)
	now := time.Now()

	s1 := &trace.Span{
		TraceID:  "trace1",
		SpanID:   "span1",
		Start:    now,
		Duration: 100, // 100ms
	}
	s2 := &trace.Span{
		TraceID:  "trace1",
		SpanID:   "span2",
		ParentID: "span1",
		Start:    now.Add(50 * time.Millisecond),
		Duration: 200, // 200ms
	}

	store.Add(s1)
	store.Add(s2)

	tr := store.GetTrace("trace1")
	if tr == nil {
		t.Fatal("GetTrace returned nil")
	}
	// Trace duration should span from earliest start to latest end.
	// s1: 0-100ms, s2: 50-250ms => total = 250ms
	if tr.Duration < 240 || tr.Duration > 260 {
		t.Errorf("trace Duration = %f, want ~250", tr.Duration)
	}
}
