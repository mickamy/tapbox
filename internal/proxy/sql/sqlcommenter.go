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

// CommentStatus indicates the result of parsing a sqlcommenter traceparent.
type CommentStatus int

const (
	// CommentAbsent means no trailing comment or no traceparent key was found.
	CommentAbsent CommentStatus = iota
	// CommentInvalid means a traceparent key was found but the value is malformed.
	CommentInvalid
	// CommentOK means a valid traceparent was extracted.
	CommentOK
)

// ParseSQLComment extracts a W3C traceparent from a trailing sqlcommenter
// comment (e.g. /*traceparent='00-...-...-01'*/). It returns CommentOK with
// the parsed result on success, CommentInvalid if a traceparent key exists
// but the value is malformed, and CommentAbsent if no traceparent key is found.
func ParseSQLComment(query string) (CommentResult, CommentStatus) {
	q := strings.TrimRight(query, " \t\n\r")
	if !strings.HasSuffix(q, "*/") {
		return CommentResult{}, CommentAbsent
	}
	start := strings.LastIndex(q, "/*")
	if start < 0 {
		return CommentResult{}, CommentAbsent
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
			return CommentResult{}, CommentInvalid
		}
		tp, ok := trace.ParseTraceparent(decoded)
		if !ok {
			return CommentResult{}, CommentInvalid
		}
		return CommentResult{TraceID: tp.TraceID, ParentID: tp.SpanID}, CommentOK
	}
	return CommentResult{}, CommentAbsent
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
