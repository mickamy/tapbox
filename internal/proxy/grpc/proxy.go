package grpc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mickamy/tapbox/internal/trace"
)

type Proxy struct {
	target    string
	conn      *grpc.ClientConn
	collector *trace.Collector

	// OnSpan is called when a new gRPC span starts with the resolved
	// traceID and spanID. This allows the SQL correlator to associate
	// subsequent SQL queries with the active gRPC span.
	OnSpan func(traceID, spanID string)
}

func NewProxy(target string, collector *trace.Collector) (*Proxy, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(RawCodec{})),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", target, err)
	}
	return &Proxy{target: target, conn: conn, collector: collector}, nil
}

func (p *Proxy) Close() error {
	if err := p.conn.Close(); err != nil {
		return fmt.Errorf("closing grpc connection: %w", err)
	}
	return nil
}

// UnknownHandler returns a grpc.StreamHandler that forwards all RPCs to the upstream.
func (p *Proxy) UnknownHandler() grpc.StreamHandler {
	return func(srv any, serverStream grpc.ServerStream) error {
		fullMethod, ok := grpc.MethodFromServerStream(serverStream)
		if !ok {
			return status.Error(codes.Internal, "failed to get method name")
		}

		start := time.Now()
		service, method := splitMethod(fullMethod)

		// Extract trace context from incoming metadata.
		traceID, parentID, spanID := extractTraceContext(serverStream)

		if p.OnSpan != nil {
			p.OnSpan(traceID, spanID)
		}

		md, _ := metadata.FromIncomingContext(serverStream.Context())
		ctx := metadata.NewOutgoingContext(serverStream.Context(), md.Copy())

		// Forward the traceparent to upstream.
		ctx = metadata.AppendToOutgoingContext(ctx, "traceparent",
			fmt.Sprintf("00-%s-%s-01", traceID, spanID))

		clientStream, err := p.conn.NewStream(ctx, &grpc.StreamDesc{
			ServerStreams: true,
			ClientStreams: true,
		}, fullMethod, grpc.ForceCodecV2(RawCodec{}))
		if err != nil {
			return p.submitSpanAt(traceID, parentID, spanID, service, method, nil, nil, md, start, time.Now(), err)
		}

		// Relay messages bidirectionally.
		var reqBytes, respBytes []byte
		var end time.Time
		errCh := make(chan error, 2)

		// Client -> Server (incoming from caller -> upstream)
		go func() {
			for {
				var msg []byte
				if recvErr := serverStream.RecvMsg(&msg); recvErr != nil {
					if errors.Is(recvErr, io.EOF) {
						if closeErr := clientStream.CloseSend(); closeErr != nil {
							log.Printf("grpc proxy: close send error: %v", closeErr)
						}
						errCh <- nil
						return
					}
					errCh <- recvErr
					return
				}
				reqBytes = msg
				if sendErr := clientStream.SendMsg(&msg); sendErr != nil {
					errCh <- sendErr
					return
				}
			}
		}()

		// Server -> Client (upstream response -> caller)
		go func() {
			for {
				var msg []byte
				if recvErr := clientStream.RecvMsg(&msg); recvErr != nil {
					if errors.Is(recvErr, io.EOF) {
						errCh <- nil
						return
					}
					errCh <- recvErr
					return
				}
				respBytes = msg
				if sendErr := serverStream.SendMsg(&msg); sendErr != nil {
					errCh <- sendErr
					return
				}
				// Capture end time after the last response is sent to the caller.
				// This must happen here (not after both goroutines finish) because
				// the client→server goroutine blocks until the caller closes its
				// stream, which occurs after the HTTP response cycle completes.
				end = time.Now()
			}
		}()

		// Wait for both relay goroutines.
		var relayErr error
		for range 2 {
			if e := <-errCh; e != nil {
				relayErr = e
			}
		}

		// Fallback for error cases where no response was ever sent.
		if end.IsZero() {
			end = time.Now()
		}

		return p.submitSpanAt(traceID, parentID, spanID, service, method, reqBytes, respBytes, md, start, end, relayErr)
	}
}

func (p *Proxy) submitSpanAt(
	traceID, parentID, spanID, service, method string,
	reqBytes, respBytes []byte,
	md metadata.MD,
	start, end time.Time,
	err error,
) error {
	duration := float64(end.Sub(start)) / float64(time.Millisecond)

	spanStatus := trace.StatusOK
	grpcCode := codes.OK.String()
	if err != nil {
		spanStatus = trace.StatusError
		if st, ok := status.FromError(err); ok {
			grpcCode = st.Code().String()
		} else {
			grpcCode = codes.Internal.String()
		}
	}

	mdMap := make(map[string]string, len(md))
	for k, v := range md {
		if len(v) > 0 {
			mdMap[k] = v[0]
		}
	}

	span := &trace.Span{
		TraceID:          traceID,
		SpanID:           spanID,
		ParentID:         parentID,
		Kind:             trace.SpanGRPC,
		Name:             service + "/" + method,
		Start:            start,
		Duration:         duration,
		Status:           spanStatus,
		GRPCService:      service,
		GRPCMethod:       method,
		GRPCCode:         grpcCode,
		GRPCMetadata:     mdMap,
		GRPCRequestBody:  hex.EncodeToString(reqBytes),
		GRPCResponseBody: hex.EncodeToString(respBytes),
	}

	p.collector.Submit(span)

	if err != nil {
		return err
	}
	return nil
}

func extractTraceContext(stream grpc.ServerStream) (traceID, parentID, spanID string) {
	spanID = trace.NewSpanID()
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return trace.NewTraceID(), "", spanID
	}
	vals := md.Get("traceparent")
	if len(vals) == 0 {
		return trace.NewTraceID(), "", spanID
	}
	parts := strings.Split(vals[0], "-")
	if len(parts) >= 4 && len(parts[1]) == 32 && len(parts[2]) == 16 {
		return parts[1], parts[2], spanID
	}
	return trace.NewTraceID(), "", spanID
}

func splitMethod(fullMethod string) (service, method string) {
	// fullMethod is like "/package.Service/Method"
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	parts := strings.SplitN(fullMethod, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return fullMethod, ""
}
