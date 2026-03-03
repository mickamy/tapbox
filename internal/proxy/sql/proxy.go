package sql

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mickamy/tapbox/internal/trace"
)

type Proxy struct {
	target     string
	listener   net.Listener
	collector  *trace.Collector
	correlator *Correlator
	protocol   DBProtocol
	connCount  atomic.Uint64
	wg         sync.WaitGroup
	closed     chan struct{}
	closeOnce  sync.Once
}

func NewProxy(target string, collector *trace.Collector, correlator *Correlator) *Proxy {
	return &Proxy{
		target:     target,
		collector:  collector,
		correlator: correlator,
		protocol:   PGProtocol{},
		closed:     make(chan struct{}),
	}
}

func (p *Proxy) Listen(ctx context.Context, addr string) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("sql proxy listen %s: %w", addr, err)
	}
	p.listener = ln
	return nil
}

func (p *Proxy) Serve() error {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.closed:
				return nil
			default:
				return fmt.Errorf("sql proxy accept: %w", err)
			}
		}
		p.wg.Add(1)
		go p.handleConn(conn)
	}
}

func (p *Proxy) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		close(p.closed)
		if err := p.listener.Close(); err != nil {
			closeErr = fmt.Errorf("closing sql proxy listener: %w", err)
		}
		p.wg.Wait()
	})
	return closeErr
}

func (p *Proxy) handleConn(clientConn net.Conn) {
	defer p.wg.Done()
	defer func() {
		if err := clientConn.Close(); err != nil {
			log.Printf("sql proxy: client close error: %v", err)
		}
	}()

	connID := p.connCount.Add(1)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	serverConn, err := dialer.Dial("tcp", p.target)
	if err != nil {
		log.Printf("sql proxy: failed to connect to %s: %v", p.target, err)
		return
	}
	defer func() {
		if err := serverConn.Close(); err != nil {
			log.Printf("sql proxy: server close error: %v", err)
		}
	}()

	// Set a deadline for the startup/auth phase to prevent hanging on
	// unresponsive servers. HandleConnection resets it after auth completes.
	authTimeout := 30 * time.Second
	_ = clientConn.SetDeadline(time.Now().Add(authTimeout))
	_ = serverConn.SetDeadline(time.Now().Add(authTimeout))

	p.protocol.HandleConnection(
		netConnAdapter{conn: clientConn},
		netConnAdapter{conn: serverConn},
		connID,
		func(ev QueryEvent) {
			var traceID, parentID string
			queryText := ev.Query
			sc, status := ParseSQLComment(ev.Query)
			switch status {
			case CommentOK:
				traceID = sc.TraceID
				parentID = sc.ParentID
				queryText = StripSQLComment(ev.Query)
			case CommentInvalid:
				traceID = trace.NewTraceID()
				queryText = StripSQLComment(ev.Query)
			case CommentAbsent:
				traceID, parentID = p.correlator.Correlate()
			}
			span := &trace.Span{
				TraceID:     traceID,
				SpanID:      trace.NewSpanID(),
				ParentID:    parentID,
				Kind:        trace.SpanSQL,
				Name:        truncateQuery(queryText, 80),
				Start:       ev.Start,
				Duration:    ev.Duration,
				SQLQuery:    queryText,
				SQLRowCount: ev.RowCount,
				SQLError:    ev.Error,
			}
			if ev.Error != "" {
				span.Status = trace.StatusError
			}
			p.collector.Submit(span)
		},
	)
}

// netConnAdapter wraps net.Conn to implement RawConn.
type netConnAdapter struct {
	conn net.Conn
}

func (n netConnAdapter) Read(b []byte) (int, error) {
	return n.conn.Read(b) //nolint:wrapcheck // wrapped by connReader
}

func (n netConnAdapter) Write(b []byte) (int, error) {
	return n.conn.Write(b) //nolint:wrapcheck // wrapped by connWriter
}

func (n netConnAdapter) Close() error {
	if err := n.conn.Close(); err != nil {
		return fmt.Errorf("closing connection: %w", err)
	}
	return nil
}

func (n netConnAdapter) SetDeadline(t time.Time) error {
	return n.conn.SetDeadline(t) //nolint:wrapcheck // pass-through
}

func truncateQuery(q string, maxLen int) string {
	if len(q) <= maxLen {
		return q
	}
	return q[:maxLen] + "..."
}
