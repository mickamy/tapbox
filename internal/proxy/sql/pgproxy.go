package sql

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"
)

// PGProtocol implements DBProtocol for PostgreSQL wire protocol.
type PGProtocol struct{}

func (PGProtocol) Name() string { return "postgres" }

// pendingQuery holds query metadata until the server sends ReadyForQuery.
type pendingQuery struct {
	query  string
	start  time.Time
	connID uint64
}

func (PGProtocol) HandleConnection(clientConn, serverConn RawConn, connID uint64, onQuery func(QueryEvent)) {
	// Phase 1: Pass through the startup handshake (SSL negotiation, startup message, auth).
	if !relayStartup(clientConn, serverConn) {
		return
	}

	// Phase 2: Proxy messages, capturing queries.
	// pending carries query info from client→server goroutine to server→client goroutine
	// so that duration can be measured when ReadyForQuery arrives.
	pending := make(chan pendingQuery, 64)
	errCh := make(chan error, 2)

	// Client -> Server: capture Query/Parse messages.
	go func() {
		err := proxyClientToServer(clientConn, serverConn, connID, pending)
		close(pending)
		errCh <- err
	}()

	// Server -> Client: emit QueryEvent with measured duration on ReadyForQuery.
	go func() {
		errCh <- proxyServerToClient(serverConn, clientConn, pending, onQuery)
	}()

	// Wait for either direction to finish.
	<-errCh
}

func relayStartup(client, server RawConn) bool {
	// Loop to handle SSL negotiation retries. When the server rejects SSL,
	// the client sends a new startup message, so we loop back to read it.
	for {
		// Read startup message length (first 4 bytes).
		header := make([]byte, 4)
		if _, err := io.ReadFull(readerFromConn(client), header); err != nil {
			return false
		}
		length := int(binary.BigEndian.Uint32(header))
		if length < 4 || length > 10240 {
			return false
		}

		// Read rest of startup message.
		msg := make([]byte, length)
		copy(msg, header)
		if _, err := io.ReadFull(readerFromConn(client), msg[4:]); err != nil {
			return false
		}

		// Check for SSLRequest (protocol version 80877103).
		if length == 8 {
			code := binary.BigEndian.Uint32(msg[4:8])
			if code == 80877103 {
				// Forward SSL request to server.
				if _, err := server.Write(msg); err != nil {
					return false
				}
				// Read server response (single byte: 'N' for no SSL, 'S' for SSL).
				resp := make([]byte, 1)
				if _, err := io.ReadFull(readerFromConn(server), resp); err != nil {
					return false
				}
				if _, err := client.Write(resp); err != nil {
					return false
				}

				// If server doesn't support SSL, client should send real startup.
				if resp[0] == 'N' {
					continue
				}
				// If SSL, we can't intercept - just pipe through.
				go func() {
					if _, err := io.Copy(writerFromConn(server), readerFromConn(client)); err != nil {
						log.Printf("pgproxy: SSL relay client->server error: %v", err)
					}
				}()
				if _, err := io.Copy(writerFromConn(client), readerFromConn(server)); err != nil {
					log.Printf("pgproxy: SSL relay server->client error: %v", err)
				}
				return false // SSL connection taken over by io.Copy.
			}
		}

		// Non-SSL startup message: forward to server and proceed to auth.
		if _, err := server.Write(msg); err != nil {
			return false
		}
		return relayAuth(client, server)
	}
}

// relayAuth relays authentication messages between client and server
// until the server sends ReadyForQuery.
func relayAuth(client, server RawConn) bool {
	for {
		msgType, payload, err := readPGMessage(server)
		if err != nil {
			return false
		}
		if err := writePGMessage(client, msgType, payload); err != nil {
			return false
		}

		// Handle auth messages that need client response.
		switch msgType {
		case 'R': // Authentication
			if len(payload) < 4 {
				continue
			}
			authType := binary.BigEndian.Uint32(payload[:4])
			switch authType {
			case 0: // AuthenticationOk
				continue
			case 12: // AuthenticationSASLFinal — server-only, no client response
				continue
			default:
				// Server needs more auth data from client (password, SASL, etc.)
				clientType, clientPayload, clientErr := readPGMessageFromClient(client)
				if clientErr != nil {
					return false
				}
				if err := writePGMessage(server, clientType, clientPayload); err != nil {
					return false
				}
			}
		case 'Z': // ReadyForQuery
			return true
		}
	}
}

func proxyClientToServer(client, server RawConn, connID uint64, pending chan<- pendingQuery) error {
	for {
		msgType, payload, err := readPGMessageFromClient(client)
		if err != nil {
			return fmt.Errorf("reading client message: %w", err)
		}

		// Capture query before forwarding.
		switch msgType {
		case 'Q': // Simple query
			query := extractString(payload)
			start := time.Now()
			if writeErr := writePGMessage(server, msgType, payload); writeErr != nil {
				return fmt.Errorf("writing query to server: %w", writeErr)
			}
			pending <- pendingQuery{query: query, start: start, connID: connID}
			continue
		case 'P': // Parse (extended query)
			query := extractParseQuery(payload)
			if query != "" {
				pending <- pendingQuery{query: query, start: time.Now(), connID: connID}
			}
		case 'X': // Terminate
			_ = writePGMessage(server, msgType, payload)
			return nil
		}

		if writeErr := writePGMessage(server, msgType, payload); writeErr != nil {
			return fmt.Errorf("writing message to server: %w", writeErr)
		}
	}
}

func proxyServerToClient(server, client RawConn, pending <-chan pendingQuery, onQuery func(QueryEvent)) error {
	for {
		msgType, payload, err := readPGMessage(server)
		if err != nil {
			return fmt.Errorf("reading server message: %w", err)
		}
		if writeErr := writePGMessage(client, msgType, payload); writeErr != nil {
			return fmt.Errorf("writing message to client: %w", writeErr)
		}

		if msgType == 'Z' { // ReadyForQuery — query cycle complete
			select {
			case pq, ok := <-pending:
				if ok {
					onQuery(QueryEvent{
						Query:    pq.query,
						Start:    pq.start,
						Duration: float64(time.Since(pq.start)) / float64(time.Millisecond),
						ConnID:   pq.connID,
					})
				}
			default:
				// No pending query (e.g. initial ReadyForQuery after auth).
			}
		}
	}
}

// readPGMessage reads a single PostgreSQL protocol message from the server.
// Format: type(1 byte) + length(4 bytes, includes self) + payload
func readPGMessage(conn RawConn) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(readerFromConn(conn), header); err != nil {
		return 0, nil, fmt.Errorf("reading pg message header: %w", err)
	}
	msgType := header[0]
	length := int(binary.BigEndian.Uint32(header[1:5]))
	if length < 4 {
		return 0, nil, fmt.Errorf("invalid pg message length %d (type %c)", length, rune(msgType))
	}
	const maxMessageSize = 64 * 1024 * 1024 // 64 MB
	if length > maxMessageSize {
		return 0, nil, fmt.Errorf("pg message too large: %d bytes (type %c)", length, rune(msgType))
	}
	payload := make([]byte, length-4)
	if _, err := io.ReadFull(readerFromConn(conn), payload); err != nil {
		return 0, nil, fmt.Errorf("reading pg message payload: %w", err)
	}
	return msgType, payload, nil
}

// readPGMessageFromClient reads a client message (same format as server messages).
func readPGMessageFromClient(conn RawConn) (byte, []byte, error) {
	return readPGMessage(conn)
}

func writePGMessage(conn RawConn, msgType byte, payload []byte) error {
	length := len(payload) + 4
	header := make([]byte, 5)
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:], uint32(length)) //nolint:gosec // length is bounded by message size
	if _, err := conn.Write(header); err != nil {
		return fmt.Errorf("writing pg message header: %w", err)
	}
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("writing pg message payload: %w", err)
		}
	}
	return nil
}

func extractString(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}

func extractParseQuery(data []byte) string {
	// Parse message format: name\0 query\0 int16 [int32...]
	// Skip the statement name.
	i := 0
	for i < len(data) && data[i] != 0 {
		i++
	}
	i++ // skip null terminator
	if i >= len(data) {
		return ""
	}
	return extractString(data[i:])
}

// connReader adapts RawConn to io.Reader.
type connReader struct{ conn RawConn }

func (r connReader) Read(b []byte) (int, error) {
	n, err := r.conn.Read(b)
	if err != nil {
		return n, fmt.Errorf("reading from connection: %w", err)
	}
	return n, nil
}

func readerFromConn(conn RawConn) io.Reader { return connReader{conn} }

// connWriter adapts RawConn to io.Writer.
type connWriter struct{ conn RawConn }

func (w connWriter) Write(b []byte) (int, error) {
	n, err := w.conn.Write(b)
	if err != nil {
		return n, fmt.Errorf("writing to connection: %w", err)
	}
	return n, nil
}

func writerFromConn(conn RawConn) io.Writer { return connWriter{conn} }
