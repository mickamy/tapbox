package sql

import "time"

// QueryEvent represents a captured SQL query.
type QueryEvent struct {
	Query    string
	Start    time.Time
	Duration float64 // milliseconds
	RowCount int
	Error    string
	ConnID   uint64
}

// DBProtocol defines the interface for database wire protocol parsing.
// This allows future support for MySQL and other databases.
type DBProtocol interface {
	// Name returns the protocol name (e.g., "postgres", "mysql").
	Name() string

	// HandleConnection processes a single client connection, parsing the wire
	// protocol and emitting QueryEvents through the callback.
	HandleConnection(clientConn, serverConn RawConn, connID uint64, onQuery func(QueryEvent))
}

// RawConn abstracts a network connection for protocol handlers.
type RawConn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
}
