package trace

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// TraceparentContext holds the trace and parent span IDs extracted from a
// W3C traceparent header.
type TraceparentContext struct {
	TraceID string
	SpanID  string
}

// ParseTraceparent extracts trace and span IDs from a W3C traceparent header.
// Format: version-traceID-spanID-flags (e.g. "00-abc...def-012...345-01")
func ParseTraceparent(header string) (TraceparentContext, bool) {
	parts := strings.Split(header, "-")
	if len(parts) < 4 {
		return TraceparentContext{}, false
	}
	traceID := parts[1]
	spanID := parts[2]
	if len(traceID) != 32 || len(spanID) != 16 {
		return TraceparentContext{}, false
	}
	if !isLowerHex(traceID) || !isLowerHex(spanID) {
		return TraceparentContext{}, false
	}
	// W3C spec: all-zero trace-id and parent-id are invalid.
	if traceID == "00000000000000000000000000000000" || spanID == "0000000000000000" {
		return TraceparentContext{}, false
	}
	return TraceparentContext{TraceID: traceID, SpanID: spanID}, true
}

// FormatTraceparent creates a W3C traceparent header value.
func FormatTraceparent(traceID, spanID string) string {
	return fmt.Sprintf("00-%s-%s-01", traceID, spanID)
}

func isLowerHex(s string) bool {
	_, err := hex.DecodeString(s)
	if err != nil {
		return false
	}
	return strings.ToLower(s) == s
}
