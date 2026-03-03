package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	notev1 "github.com/mickamy/tapbox/example/gen/note/v1"
	"github.com/mickamy/tapbox/example/internal/db"
	"github.com/mickamy/tapbox/example/internal/env"
	"github.com/mickamy/tapbox/example/internal/tracing"
)

type noteServer struct {
	notev1.UnimplementedNoteServiceServer

	pool *pgxpool.Pool
}

func (s *noteServer) CreateNote(ctx context.Context, req *notev1.CreateNoteRequest) (*notev1.Note, error) {
	if req.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	tp := tracing.FromGRPCContext(ctx)
	var note notev1.Note
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		tracing.AppendTraceparent(
			"INSERT INTO notes (title, body) VALUES ($1, $2) RETURNING id, title, body, created_at",
			tp,
		),
		req.GetTitle(), req.GetBody(),
	).Scan(&note.Id, &note.Title, &note.Body, &createdAt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert: %v", err)
	}
	note.CreatedAt = createdAt.Format(time.RFC3339)
	return &note, nil
}

func (s *noteServer) GetNote(ctx context.Context, req *notev1.GetNoteRequest) (*notev1.Note, error) {
	tp := tracing.FromGRPCContext(ctx)
	var note notev1.Note
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		tracing.AppendTraceparent(
			"SELECT id, title, body, created_at FROM notes WHERE id = $1",
			tp,
		),
		req.GetId(),
	).Scan(&note.Id, &note.Title, &note.Body, &createdAt)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "note %d not found", req.GetId())
	}
	note.CreatedAt = createdAt.Format(time.RFC3339)
	return &note, nil
}

func (s *noteServer) ListNotes(ctx context.Context, _ *notev1.ListNotesRequest) (*notev1.ListNotesResponse, error) {
	tp := tracing.FromGRPCContext(ctx)
	rows, err := s.pool.Query(ctx,
		tracing.AppendTraceparent(
			"SELECT id, title, body, created_at FROM notes ORDER BY created_at DESC LIMIT 50",
			tp,
		),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query: %v", err)
	}
	defer rows.Close()

	var notes []*notev1.Note
	for rows.Next() {
		var n notev1.Note
		var createdAt time.Time
		if err := rows.Scan(&n.Id, &n.Title, &n.Body, &createdAt); err != nil {
			return nil, status.Errorf(codes.Internal, "scan: %v", err)
		}
		n.CreatedAt = createdAt.Format(time.RFC3339)
		notes = append(notes, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "rows: %v", err)
	}

	return &notev1.ListNotesResponse{Notes: notes}, nil
}

func main() {
	dbDSN := env.Or("DB_DSN", "postgres://tapbox:tapbox@localhost:5433/tapbox_example?sslmode=disable")
	grpcAddr := env.Or("GRPC_ADDR", ":50051")

	log.Printf("Backend starting (DB: %s)", dbDSN)
	ctx := context.Background()
	pool, err := db.ConnectWithRetry(ctx, dbDSN, 30)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listen: %v", err) //nolint:gocritic // main exit
	}

	srv := grpc.NewServer()
	notev1.RegisterNoteServiceServer(srv, &noteServer{pool: pool})
	reflection.Register(srv)

	log.Printf("Backend gRPC server listening on %s", grpcAddr)
	if err := srv.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
