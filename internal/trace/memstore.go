package trace

import (
	"sync"
	"time"
)

type MemStore struct {
	mu          sync.RWMutex
	traces      map[string]*Trace
	order       []string // ring buffer of trace IDs
	maxTraces   int
	pos         int
	full        bool
	subscribers map[chan *Span]struct{}
}

func NewMemStore(maxTraces int) *MemStore {
	return &MemStore{
		traces:      make(map[string]*Trace),
		order:       make([]string, maxTraces),
		maxTraces:   maxTraces,
		subscribers: make(map[chan *Span]struct{}),
	}
}

func (m *MemStore) Add(span *Span) {
	m.mu.Lock()

	t, ok := m.traces[span.TraceID]
	if !ok {
		// Evict the oldest trace if ring buffer is full.
		if m.full {
			oldID := m.order[m.pos]
			delete(m.traces, oldID)
		}
		t = &Trace{
			TraceID: span.TraceID,
			Start:   span.Start,
		}
		m.traces[span.TraceID] = t
		m.order[m.pos] = span.TraceID
		m.pos++
		if m.pos >= m.maxTraces {
			m.pos = 0
			m.full = true
		}
	}

	t.Spans = append(t.Spans, span)
	updateTraceTiming(t)

	// Copy subscriber list under lock, then notify without lock.
	subs := make([]chan *Span, 0, len(m.subscribers))
	for ch := range m.subscribers {
		subs = append(subs, ch)
	}
	m.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- span:
		default:
			// Drop if subscriber is slow.
		}
	}
}

func (m *MemStore) GetTrace(traceID string) *Trace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.traces[traceID]
}

func (m *MemStore) ListTraces(offset, limit int) []*Trace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect traces in reverse insertion order (newest first).
	total := len(m.traces)
	if total == 0 || offset >= total {
		return nil
	}

	ids := m.orderedIDs()
	// Reverse to get newest first.
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}

	if offset > 0 {
		if offset >= len(ids) {
			return nil
		}
		ids = ids[offset:]
	}
	if limit > 0 && limit < len(ids) {
		ids = ids[:limit]
	}

	result := make([]*Trace, 0, len(ids))
	for _, id := range ids {
		if t, ok := m.traces[id]; ok {
			result = append(result, t)
		}
	}
	return result
}

func (m *MemStore) Subscribe() <-chan *Span {
	ch := make(chan *Span, 64)
	m.mu.Lock()
	m.subscribers[ch] = struct{}{}
	m.mu.Unlock()
	return ch
}

func (m *MemStore) Unsubscribe(ch <-chan *Span) {
	m.mu.Lock()
	for bch := range m.subscribers {
		if (<-chan *Span)(bch) == ch {
			delete(m.subscribers, bch)
			close(bch)
			break
		}
	}
	m.mu.Unlock()
}

func updateTraceTiming(t *Trace) {
	var earliest time.Time
	var latestEnd time.Time
	for _, s := range t.Spans {
		if earliest.IsZero() || s.Start.Before(earliest) {
			earliest = s.Start
		}
		end := s.Start.Add(time.Duration(s.Duration * float64(time.Millisecond)))
		if end.After(latestEnd) {
			latestEnd = end
		}
	}
	t.Start = earliest
	t.Duration = float64(latestEnd.Sub(earliest)) / float64(time.Millisecond)
}

func (m *MemStore) orderedIDs() []string {
	if m.full {
		ids := make([]string, 0, m.maxTraces)
		for i := range m.maxTraces {
			idx := (m.pos + i) % m.maxTraces
			if m.order[idx] != "" {
				ids = append(ids, m.order[idx])
			}
		}
		return ids
	}
	ids := make([]string, 0, m.pos)
	for i := range m.pos {
		if m.order[i] != "" {
			ids = append(ids, m.order[i])
		}
	}
	return ids
}

var _ Store = (*MemStore)(nil)
