package http

import (
	"bytes"
	"fmt"
	"io"
	"log"
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

	reqBody := captureBody(r.Body, p.maxBodySize)
	r.Body = io.NopCloser(bytes.NewReader(reqBody))
	r.ContentLength = int64(len(reqBody))

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

func captureBody(rc io.ReadCloser, maxSize int) []byte {
	if rc == nil {
		return nil
	}
	defer func() {
		if err := rc.Close(); err != nil {
			log.Printf("captureBody: close error: %v", err)
		}
	}()
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, io.LimitReader(rc, int64(maxSize))); err != nil {
		return nil
	}
	return buf.Bytes()
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
