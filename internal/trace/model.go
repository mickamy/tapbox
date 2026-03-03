package trace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type SpanKind int

const (
	SpanHTTP SpanKind = iota
	SpanConnect
	SpanGRPC
	SpanSQL
)

func (k SpanKind) String() string {
	switch k {
	case SpanHTTP:
		return "http"
	case SpanConnect:
		return "connect"
	case SpanGRPC:
		return "grpc"
	case SpanSQL:
		return "sql"
	default:
		return "unknown"
	}
}

func (k SpanKind) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(k.String())
	if err != nil {
		return nil, fmt.Errorf("marshaling span kind: %w", err)
	}
	return b, nil
}

func (k *SpanKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("unmarshaling span kind: %w", err)
	}
	switch s {
	case "http":
		*k = SpanHTTP
	case "connect":
		*k = SpanConnect
	case "grpc":
		*k = SpanGRPC
	case "sql":
		*k = SpanSQL
	default:
		return fmt.Errorf("unknown span kind: %q", s)
	}
	return nil
}

type SpanStatus int

const (
	StatusOK SpanStatus = iota
	StatusError
)

func (s SpanStatus) String() string {
	if s == StatusError {
		return "error"
	}
	return "ok"
}

func (s SpanStatus) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(s.String())
	if err != nil {
		return nil, fmt.Errorf("marshaling span status: %w", err)
	}
	return b, nil
}

func (s *SpanStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("unmarshaling span status: %w", err)
	}
	switch str {
	case "ok":
		*s = StatusOK
	case "error":
		*s = StatusError
	default:
		return fmt.Errorf("unknown span status: %q", str)
	}
	return nil
}

type Span struct {
	TraceID  string     `json:"trace_id"`
	SpanID   string     `json:"span_id"`
	ParentID string     `json:"parent_id,omitempty"`
	Kind     SpanKind   `json:"kind"`
	Name     string     `json:"name"`
	Start    time.Time  `json:"start"`
	Duration float64    `json:"duration_ms"`
	Status   SpanStatus `json:"status"`

	// HTTP fields
	HTTPMethod       string            `json:"http_method,omitempty"`
	HTTPPath         string            `json:"http_path,omitempty"`
	HTTPStatusCode   int               `json:"http_status_code,omitempty"`
	HTTPRequestBody  string            `json:"http_request_body,omitempty"`
	HTTPResponseBody string            `json:"http_response_body,omitempty"`
	HTTPHeaders      map[string]string `json:"http_headers,omitempty"`

	// gRPC fields
	GRPCService      string            `json:"grpc_service,omitempty"`
	GRPCMethod       string            `json:"grpc_method,omitempty"`
	GRPCCode         string            `json:"grpc_code,omitempty"`
	GRPCMetadata     map[string]string `json:"grpc_metadata,omitempty"`
	GRPCRequestBody  string            `json:"grpc_request_body,omitempty"`
	GRPCResponseBody string            `json:"grpc_response_body,omitempty"`

	// SQL fields
	SQLQuery    string `json:"sql_query,omitempty"`
	SQLArgs     string `json:"sql_args,omitempty"`
	SQLRowCount int    `json:"sql_row_count,omitempty"`
	SQLError    string `json:"sql_error,omitempty"`

	// Connect RPC fields
	ConnectService      string            `json:"connect_service,omitempty"`
	ConnectMethod       string            `json:"connect_method,omitempty"`
	ConnectContentType  string            `json:"connect_content_type,omitempty"`
	ConnectTimeoutMs    string            `json:"connect_timeout_ms,omitempty"`
	ConnectHTTPStatus   int               `json:"connect_http_status,omitempty"`
	ConnectErrorCode    string            `json:"connect_error_code,omitempty"`
	ConnectErrorMessage string            `json:"connect_error_message,omitempty"`
	ConnectHeaders      map[string]string `json:"connect_headers,omitempty"`
	ConnectRequestBody  string            `json:"connect_request_body,omitempty"`
	ConnectResponseBody string            `json:"connect_response_body,omitempty"`
	ConnectIsStreaming  bool              `json:"connect_is_streaming,omitempty"`
}

type Trace struct {
	TraceID  string    `json:"trace_id"`
	Spans    []*Span   `json:"spans"`
	Start    time.Time `json:"start"`
	Duration float64   `json:"duration_ms"`
}

func (t *Trace) RootSpan() *Span {
	for _, s := range t.Spans {
		if s.ParentID == "" {
			return s
		}
	}
	if len(t.Spans) > 0 {
		return t.Spans[0]
	}
	return nil
}

func (t *Trace) snapshot() *Trace {
	cp := *t
	cp.Spans = make([]*Span, len(t.Spans))
	for i, s := range t.Spans {
		clone := *s
		clone.HTTPHeaders = cloneMap(s.HTTPHeaders)
		clone.GRPCMetadata = cloneMap(s.GRPCMetadata)
		clone.ConnectHeaders = cloneMap(s.ConnectHeaders)
		cp.Spans[i] = &clone
	}
	return &cp
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func NewTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

func NewSpanID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}
