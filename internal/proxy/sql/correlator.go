package sql

import (
	"sync"
	"time"

	"github.com/mickamy/tapbox/internal/trace"
)

// Correlator associates SQL spans with the most recently active HTTP/gRPC
// trace. It maintains a stack of active contexts so that concurrent requests
// are handled correctly: each SetActive pushes a context, ClearActive removes
// it, and Correlate returns the most recent non-expired entry.
type Correlator struct {
	mu      sync.Mutex
	entries []correlatorEntry
	ttl     time.Duration
}

type correlatorEntry struct {
	traceID string
	spanID  string
	seen    time.Time
}

const defaultTTL = 5 * time.Second

func NewCorrelator(ttl time.Duration) *Correlator {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Correlator{
		ttl: ttl,
	}
}

// SetActive pushes a trace context onto the stack, typically called when an
// HTTP or gRPC span begins.
func (c *Correlator) SetActive(traceID, spanID string) {
	c.mu.Lock()
	c.entries = append(c.entries, correlatorEntry{
		traceID: traceID,
		spanID:  spanID,
		seen:    time.Now(),
	})
	c.mu.Unlock()
}

// ClearActive removes the entry matching the given traceID and spanID.
// Called when an HTTP or gRPC span completes.
func (c *Correlator) ClearActive(traceID, spanID string) {
	c.mu.Lock()
	for i := len(c.entries) - 1; i >= 0; i-- {
		if c.entries[i].traceID == traceID && c.entries[i].spanID == spanID {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			break
		}
	}
	c.mu.Unlock()
}

// Correlate returns the trace and parent span ID for a SQL query.
// It returns the most recent non-expired entry from the stack.
// If no active trace is found within the TTL, new IDs are generated.
func (c *Correlator) Correlate() (traceID, parentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	// Search from the end (most recent) for a non-expired entry.
	for i := len(c.entries) - 1; i >= 0; i-- {
		if now.Sub(c.entries[i].seen) < c.ttl {
			return c.entries[i].traceID, c.entries[i].spanID
		}
	}
	return trace.NewTraceID(), ""
}
