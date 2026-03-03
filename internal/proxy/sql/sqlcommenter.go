package sql

import (
	"net/url"
	"strings"

	"github.com/mickamy/tapbox/internal/trace"
)

// CommentResult holds the parsed traceparent from a sqlcommenter comment.
type CommentResult struct {
	TraceID  string
	ParentID string
}

// ParseSQLComment extracts a W3C traceparent from a trailing sqlcommenter
// comment (e.g. /*traceparent='00-...-...-01'*/). It returns false if the
// query has no trailing comment, no traceparent key, or an invalid value.
func ParseSQLComment(query string) (CommentResult, bool) {
	q := strings.TrimRight(query, " \t\n\r")
	if !strings.HasSuffix(q, "*/") {
		return CommentResult{}, false
	}
	start := strings.LastIndex(q, "/*")
	if start < 0 {
		return CommentResult{}, false
	}
	body := q[start+2 : len(q)-2]

	for pair := range strings.SplitSeq(body, ",") {
		pair = strings.TrimSpace(pair)
		key, val, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key != "traceparent" {
			continue
		}
		val = strings.Trim(val, "'")
		decoded, err := url.QueryUnescape(val)
		if err != nil {
			return CommentResult{}, false
		}
		tp, ok := trace.ParseTraceparent(decoded)
		if !ok {
			return CommentResult{}, false
		}
		return CommentResult{TraceID: tp.TraceID, ParentID: tp.SpanID}, true
	}
	return CommentResult{}, false
}

// StripSQLComment removes a trailing sqlcommenter comment from the query.
// Non-trailing comments (e.g. optimizer hints) are preserved.
func StripSQLComment(query string) string {
	q := strings.TrimRight(query, " \t\n\r")
	if !strings.HasSuffix(q, "*/") {
		return query
	}
	start := strings.LastIndex(q, "/*")
	if start < 0 {
		return query
	}
	return strings.TrimRight(q[:start], " \t\n\r")
}
