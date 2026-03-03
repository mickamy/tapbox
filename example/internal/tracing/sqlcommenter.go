package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"google.golang.org/grpc/metadata"
)

// AppendTraceparent appends a sqlcommenter traceparent comment to a SQL query.
// If traceparent is empty, the query is returned unchanged.
func AppendTraceparent(query, traceparent string) string {
	if traceparent == "" {
		return query
	}
	return fmt.Sprintf("%s /*traceparent='%s'*/", query, url.QueryEscape(traceparent))
}

// FromGRPCContext extracts the traceparent value from incoming gRPC metadata.
func FromGRPCContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("traceparent")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// FromHTTPRequest extracts the traceparent value from an HTTP request header.
func FromHTTPRequest(r *http.Request) string {
	return r.Header.Get("Traceparent")
}
