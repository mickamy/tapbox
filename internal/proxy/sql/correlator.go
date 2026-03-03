package sql

import (
	"sync"
	"time"

	"github.com/mickamy/tapbox/internal/trace"
)

// Correlator assigns SQL spans to the most recent active trace.
// In local development with low concurrency this heuristic works well.
type Correlator struct {
	mu          sync.Mutex
	activeTrace string // most recently seen trace ID
	activeSpan  string // most recently seen span ID
	lastSeen    time.Time
	ttl         time.Duration
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

// SetActive records the currently active trace context, typically called
// when an HTTP or gRPC span begins.
func (c *Correlator) SetActive(traceID, spanID string) {
	c.mu.Lock()
	c.activeTrace = traceID
	c.activeSpan = spanID
	c.lastSeen = time.Now()
	c.mu.Unlock()
}

// Correlate returns the trace and parent span ID for a SQL query.
// If no active trace is found within the TTL, new IDs are generated.
func (c *Correlator) Correlate() (traceID, parentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTrace != "" && time.Since(c.lastSeen) < c.ttl {
		return c.activeTrace, c.activeSpan
	}
	return trace.NewTraceID(), ""
}
