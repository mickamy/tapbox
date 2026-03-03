package http

import (
	"fmt"
	"strings"

	"github.com/mickamy/tapbox/internal/trace"
)

const traceparentHeader = "Traceparent"

type traceContext struct {
	TraceID string
	SpanID  string
}

// parseTraceparent extracts trace and span IDs from W3C traceparent header.
// Format: version-traceID-spanID-flags (e.g. "00-abc...def-012...345-01")
func parseTraceparent(header string) (traceContext, bool) {
	parts := strings.Split(header, "-")
	if len(parts) < 4 {
		return traceContext{}, false
	}
	traceID := parts[1]
	spanID := parts[2]
	if len(traceID) != 32 || len(spanID) != 16 {
		return traceContext{}, false
	}
	return traceContext{TraceID: traceID, SpanID: spanID}, true
}

// formatTraceparent creates a W3C traceparent header value.
func formatTraceparent(traceID, spanID string) string {
	return fmt.Sprintf("00-%s-%s-01", traceID, spanID)
}

// extractOrCreate reads the traceparent header from the request, or creates new IDs.
func extractOrCreate(header string) (traceID, parentID, spanID string) {
	spanID = trace.NewSpanID()
	if header != "" {
		if tc, ok := parseTraceparent(header); ok {
			return tc.TraceID, tc.SpanID, spanID
		}
	}
	return trace.NewTraceID(), "", spanID
}
