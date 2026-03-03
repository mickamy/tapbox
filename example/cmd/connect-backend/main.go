package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	notev1 "github.com/mickamy/tapbox/example/gen/note/v1"
	"github.com/mickamy/tapbox/example/gen/note/v1/notev1connect"
	"github.com/mickamy/tapbox/example/internal/db"
	"github.com/mickamy/tapbox/example/internal/env"
	"github.com/mickamy/tapbox/example/internal/tracing"
)

type noteServer struct {
	pool *pgxpool.Pool
}

func (s *noteServer) CreateNote(
	ctx context.Context,
	req *connect.Request[notev1.CreateNoteRequest],
) (*connect.Response[notev1.Note], error) {
	if req.Msg.GetTitle() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title is required"))
	}

	tp := req.Header().Get("Traceparent")
	var note notev1.Note
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		tracing.AppendTraceparent(
			"INSERT INTO notes (title, body) VALUES ($1, $2) RETURNING id, title, body, created_at",
			tp,
		),
		req.Msg.GetTitle(), req.Msg.GetBody(),
	).Scan(&note.Id, &note.Title, &note.Body, &createdAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("insert: %w", err))
	}
	note.CreatedAt = createdAt.Format(time.RFC3339)
	return connect.NewResponse(&note), nil
}

func (s *noteServer) GetNote(
	ctx context.Context,
	req *connect.Request[notev1.GetNoteRequest],
) (*connect.Response[notev1.Note], error) {
	tp := req.Header().Get("Traceparent")
	var note notev1.Note
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		tracing.AppendTraceparent(
			"SELECT id, title, body, created_at FROM notes WHERE id = $1",
			tp,
		),
		req.Msg.GetId(),
	).Scan(&note.Id, &note.Title, &note.Body, &createdAt)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeNotFound,
			fmt.Errorf("note %d not found", req.Msg.GetId()),
		)
	}
	note.CreatedAt = createdAt.Format(time.RFC3339)
	return connect.NewResponse(&note), nil
}

func (s *noteServer) ListNotes(
	ctx context.Context,
	req *connect.Request[notev1.ListNotesRequest],
) (*connect.Response[notev1.ListNotesResponse], error) {
	tp := req.Header().Get("Traceparent")
	rows, err := s.pool.Query(ctx,
		tracing.AppendTraceparent(
			"SELECT id, title, body, created_at FROM notes ORDER BY created_at DESC LIMIT 50",
			tp,
		),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("query: %w", err))
	}
	defer rows.Close()

	var notes []*notev1.Note
	for rows.Next() {
		var n notev1.Note
		var createdAt time.Time
		if err := rows.Scan(&n.Id, &n.Title, &n.Body, &createdAt); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("scan: %w", err))
		}
		n.CreatedAt = createdAt.Format(time.RFC3339)
		notes = append(notes, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("rows: %w", err))
	}

	return connect.NewResponse(&notev1.ListNotesResponse{Notes: notes}), nil
}

func main() {
	dbDSN := env.Or("DB_DSN", "postgres://tapbox:tapbox@localhost:5433/tapbox_example?sslmode=disable")
	addr := env.Or("CONNECT_ADDR", ":50052")

	log.Printf("Connect backend starting (DB: %s)", dbDSN)

	ctx := context.Background()
	pool, err := db.ConnectWithRetry(ctx, dbDSN, 30)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	path, handler := notev1connect.NewNoteServiceHandler(&noteServer{pool: pool})
	mux.Handle(path, handler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Connect backend listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err) //nolint:gocritic // main exit
	}
}
