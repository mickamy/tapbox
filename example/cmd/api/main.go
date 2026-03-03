package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"connectrpc.com/connect"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	notev1 "github.com/mickamy/tapbox/example/gen/note/v1"
	"github.com/mickamy/tapbox/example/gen/note/v1/notev1connect"
	"github.com/mickamy/tapbox/example/internal/env"
)

func main() {
	grpcTarget := env.Or("GRPC_TARGET", "localhost:9090")
	connectTarget := env.Or("CONNECT_TARGET", "http://localhost:8080")
	httpBackend := env.Or("HTTP_BACKEND", "http://localhost:8080")
	connectBackend := env.Or("CONNECT_BACKEND", "http://localhost:50052")
	searchBackend := env.Or("SEARCH_BACKEND", "http://localhost:4000")
	httpAddr := env.Or("HTTP_ADDR", ":3000")

	// Parse URLs before creating resources with defers.
	httpBackendURL, err := url.Parse(httpBackend)
	if err != nil {
		log.Fatalf("parsing http backend: %v", err)
	}

	connectBackendURL, err := url.Parse(connectBackend)
	if err != nil {
		log.Fatalf("parsing connect backend: %v", err)
	}

	searchBackendURL, err := url.Parse(searchBackend)
	if err != nil {
		log.Fatalf("parsing search backend: %v", err)
	}

	conn, err := grpc.NewClient(grpcTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("grpc dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	grpcClient := notev1.NewNoteServiceClient(conn)
	connectClient := notev1connect.NewNoteServiceClient(http.DefaultClient, connectTarget)

	mux := http.NewServeMux()

	// Composite endpoint: calls gRPC, Connect, and HTTP backends via tapbox.
	mux.HandleFunc("POST /notes", handleCreateNote(grpcClient, connectClient, httpBackendURL))

	// Simple gRPC-only endpoints.
	mux.HandleFunc("GET /notes/{id}", handleGetNote(grpcClient))
	mux.HandleFunc("GET /notes", handleListNotes(grpcClient))

	// Reverse proxy: Connect requests forwarded by tapbox → connect-backend.
	mux.Handle("/note.v1.NoteService/", reverseProxy(connectBackendURL))

	// Reverse proxy: search requests forwarded by tapbox → http-backend.
	mux.Handle("/search", reverseProxy(searchBackendURL))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("API server listening on %s (gRPC: %s, Connect: %s, HTTP: %s)",
		httpAddr, grpcTarget, connectTarget, httpBackend)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err) //nolint:gocritic // main exit
	}
}

func reverseProxy(target *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
		},
	}
}

type compositeResponse struct {
	Created *notev1.Note   `json:"created"`
	Notes   []*notev1.Note `json:"notes"`
	Related []searchResult `json:"related"`
}

type searchResult struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// handleCreateNote is a composite handler that fans out to all three backends.
//
// Trace tree (all hops through tapbox):
//
//	POST /notes  (client → tapbox HTTP → api)
//	 ├── gRPC CreateNote    (api → tapbox gRPC → grpc-backend → SQL INSERT)
//	 ├── Connect ListNotes  (api → tapbox HTTP → api proxy → connect-backend → SQL SELECT)
//	 └── HTTP GET /search   (api → tapbox HTTP → api proxy → http-backend → SQL SELECT)
func handleCreateNote(
	grpcClient notev1.NoteServiceClient,
	connectClient notev1connect.NoteServiceClient,
	httpBackend *url.URL,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		tp := r.Header.Get("Traceparent")
		ctx := r.Context()

		// 1. Create note via gRPC backend.
		grpcCtx := ctx
		if tp != "" {
			md := metadata.Pairs("traceparent", tp)
			grpcCtx = metadata.NewOutgoingContext(ctx, md)
		}
		note, err := grpcClient.CreateNote(grpcCtx, &notev1.CreateNoteRequest{
			Title: body.Title,
			Body:  body.Body,
		})
		if err != nil {
			log.Printf("gRPC CreateNote: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// 2. List notes via Connect backend.
		connectReq := connect.NewRequest(&notev1.ListNotesRequest{})
		if tp != "" {
			connectReq.Header().Set("Traceparent", tp)
		}
		listResp, err := connectClient.ListNotes(ctx, connectReq)
		if err != nil {
			log.Printf("Connect ListNotes: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// 3. Search related notes via HTTP backend.
		related := searchHTTPBackend(ctx, httpBackend, body.Title, tp)

		writeJSON(w, http.StatusCreated, compositeResponse{
			Created: note,
			Notes:   listResp.Msg.GetNotes(),
			Related: related,
		})
	}
}

// searchHTTPBackend calls the HTTP backend's /search endpoint.
// Errors are logged but not propagated; an empty slice is returned on failure.
func searchHTTPBackend(ctx context.Context, backend *url.URL, query, traceparent string) []searchResult {
	target := *backend
	target.Path = "/search"
	target.RawQuery = "q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		log.Printf("HTTP search request: %v", err)
		return nil
	}
	if traceparent != "" {
		req.Header.Set("Traceparent", traceparent)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // URL from config
	if err != nil {
		log.Printf("HTTP search: %v", err)
		return nil
	}
	defer resp.Body.Close() //nolint:errcheck

	var results []searchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		log.Printf("HTTP search decode: %v", err)
		return nil
	}
	return results
}

func handleGetNote(client notev1.NoteServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
			return
		}

		r = withTraceparent(r)
		note, err := client.GetNote(r.Context(), &notev1.GetNoteRequest{Id: id}) //nolint:contextcheck
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, note)
	}
}

func handleListNotes(client notev1.NoteServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r = withTraceparent(r)
		resp, err := client.ListNotes(r.Context(), &notev1.ListNotesRequest{}) //nolint:contextcheck
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp.GetNotes())
	}
}

func withTraceparent(r *http.Request) *http.Request {
	tp := r.Header.Get("Traceparent")
	if tp == "" {
		return r
	}
	md := metadata.Pairs("traceparent", tp)
	ctx := metadata.NewOutgoingContext(r.Context(), md)
	return r.WithContext(ctx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}
