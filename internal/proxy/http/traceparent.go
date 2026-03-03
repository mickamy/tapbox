package http

import (
	"github.com/mickamy/tapbox/internal/trace"
)

const traceparentHeader = "Traceparent"

// extractOrCreate reads the traceparent header from the request, or creates new IDs.
func extractOrCreate(header string) (traceID, parentID, spanID string) {
	spanID = trace.NewSpanID()
	if header != "" {
		if tc, ok := trace.ParseTraceparent(header); ok {
			return tc.TraceID, tc.SpanID, spanID
		}
	}
	return trace.NewTraceID(), "", spanID
}

// formatTraceparent creates a W3C traceparent header value.
func formatTraceparent(traceID, spanID string) string {
	return trace.FormatTraceparent(traceID, spanID)
}
