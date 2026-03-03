package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/mickamy/tapbox/internal/trace"
)

type Proxy struct {
	rp          *httputil.ReverseProxy
	collector   *trace.Collector
	maxBodySize int

	// OnSpan is called when a new HTTP/Connect span starts with the resolved
	// traceID and spanID. This allows the caller (e.g. SQL correlator) to
	// associate subsequent SQL queries with the active trace.
	OnSpan func(traceID, spanID string)
}

func NewProxy(target string, collector *trace.Collector, maxBodySize int) (*Proxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parsing target URL: %w", err)
	}

	p := &Proxy{
		collector:   collector,
		maxBodySize: maxBodySize,
	}

	p.rp = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)
			pr.Out.Host = u.Host
		},
	}

	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	traceID, parentID, spanID := extractOrCreate(r.Header.Get(traceparentHeader))

	if p.OnSpan != nil {
		p.OnSpan(traceID, spanID)
	}

	// Inject traceparent into the outgoing request so the upstream can propagate it.
	r.Header.Set(traceparentHeader, formatTraceparent(traceID, spanID))

	var reqBody []byte
	reqBody, r.Body, r.ContentLength = captureBody(r.Body, p.maxBodySize)

	rec := &responseRecorder{
		ResponseWriter: w,
		maxBodySize:    p.maxBodySize,
		body:           &bytes.Buffer{},
	}

	p.rp.ServeHTTP(rec, r) //nolint:gosec // target URL is configured by the user at startup, not from external input

	duration := float64(time.Since(start)) / float64(time.Millisecond)

	if isConnectRequest(r.Method, r.URL.Path, r.Header.Get("Content-Type")) {
		p.submitConnectSpan(r, rec, traceID, parentID, spanID, start, duration, reqBody)
	} else {
		p.submitHTTPSpan(r, rec, traceID, parentID, spanID, start, duration, reqBody)
	}
}

func (p *Proxy) submitHTTPSpan(
	r *http.Request, rec *responseRecorder,
	traceID, parentID, spanID string,
	start time.Time, duration float64, reqBody []byte,
) {
	status := trace.StatusOK
	if rec.statusCode >= 400 {
		status = trace.StatusError
	}

	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	span := &trace.Span{
		TraceID:          traceID,
		SpanID:           spanID,
		ParentID:         parentID,
		Kind:             trace.SpanHTTP,
		Name:             r.Method + " " + r.URL.Path,
		Start:            start,
		Duration:         duration,
		Status:           status,
		HTTPMethod:       r.Method,
		HTTPPath:         r.URL.Path,
		HTTPStatusCode:   rec.statusCode,
		HTTPHeaders:      headers,
		HTTPRequestBody:  string(reqBody),
		HTTPResponseBody: rec.body.String(),
	}

	p.collector.Submit(span)
}

func (p *Proxy) submitConnectSpan(
	r *http.Request, rec *responseRecorder,
	traceID, parentID, spanID string,
	start time.Time, duration float64, reqBody []byte,
) {
	service, method := parseConnectPath(r.URL.Path)
	ct := r.Header.Get("Content-Type")

	status := trace.StatusOK
	if rec.statusCode >= 400 {
		status = trace.StatusError
	}

	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	respBody := rec.body.Bytes()
	var errCode, errMsg string
	if rec.statusCode >= 400 {
		errCode, errMsg = parseConnectError(respBody)
	}

	span := &trace.Span{
		TraceID:             traceID,
		SpanID:              spanID,
		ParentID:            parentID,
		Kind:                trace.SpanConnect,
		Name:                service + "/" + method,
		Start:               start,
		Duration:            duration,
		Status:              status,
		ConnectService:      service,
		ConnectMethod:       method,
		ConnectContentType:  ct,
		ConnectTimeoutMs:    r.Header.Get("Connect-Timeout-Ms"),
		ConnectHTTPStatus:   rec.statusCode,
		ConnectErrorCode:    errCode,
		ConnectErrorMessage: errMsg,
		ConnectHeaders:      headers,
		ConnectRequestBody:  string(reqBody),
		ConnectResponseBody: string(respBody),
		ConnectIsStreaming:  isConnectStreaming(ct),
	}

	p.collector.Submit(span)
}

// captureBody reads up to maxCapture bytes for span capture and reconstructs
// the request body so the full content (including any remainder beyond the
// capture limit) is forwarded to the upstream server. This prevents unbounded
// memory allocation from arbitrarily large request bodies.
func captureBody(rc io.ReadCloser, maxCapture int) (captured []byte, body io.ReadCloser, contentLength int64) {
	if rc == nil {
		return nil, nil, 0
	}

	head, err := io.ReadAll(io.LimitReader(rc, int64(maxCapture)))
	if err != nil {
		// On read error, return what we have and let upstream handle it.
		_ = rc.Close()
		return head, io.NopCloser(bytes.NewReader(head)), int64(len(head))
	}

	// Check if there is more data beyond the capture limit.
	// If so, concatenate captured bytes with the remaining stream.
	var probe [1]byte
	n, _ := rc.Read(probe[:])
	if n == 0 {
		// Body fits within maxCapture; no remainder.
		_ = rc.Close()
		return head, io.NopCloser(bytes.NewReader(head)), int64(len(head))
	}

	// Body exceeds maxCapture: stream the remainder without buffering it all.
	// Note: n>0 with probeErr (e.g. io.EOF) is valid — the probed byte must
	// still be included in the forwarded body.
	remainder := io.MultiReader(bytes.NewReader(probe[:n]), rc)
	combined := readCloser{
		Reader: io.MultiReader(bytes.NewReader(head), remainder),
		close:  rc.Close,
	}
	return head, combined, -1 // -1 = chunked / unknown length
}

// readCloser wraps an io.Reader with a custom Close function.
type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error {
	return r.close()
}

type responseRecorder struct {
	http.ResponseWriter

	statusCode  int
	maxBodySize int
	body        *bytes.Buffer
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.statusCode = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.statusCode = http.StatusOK
		r.wroteHeader = true
	}
	if r.body.Len() < r.maxBodySize {
		remaining := r.maxBodySize - r.body.Len()
		if len(b) > remaining {
			r.body.Write(b[:remaining])
		} else {
			r.body.Write(b)
		}
	}
	n, err := r.ResponseWriter.Write(b)
	if err != nil {
		return n, fmt.Errorf("writing response: %w", err)
	}
	return n, nil
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
