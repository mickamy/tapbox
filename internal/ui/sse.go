package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/mickamy/tapbox/internal/trace"
)

type sseMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Hub struct {
	store   trace.Store
	mu      sync.Mutex
	clients map[*sseClient]struct{}
}

type sseClient struct {
	ch     chan []byte
	cancel context.CancelFunc
}

func NewHub(store trace.Store) *Hub {
	return &Hub{
		store:   store,
		clients: make(map[*sseClient]struct{}),
	}
}

func (h *Hub) Start(ctx context.Context) {
	ch := h.store.Subscribe()
	go func() {
		defer h.store.Unsubscribe(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case span, ok := <-ch:
				if !ok {
					return
				}
				h.broadcast(span)
			}
		}
	}()
}

func (h *Hub) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	client := &sseClient{ch: make(chan []byte, 64), cancel: cancel}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-client.ch:
			_, err := fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Hub) broadcast(span *trace.Span) {
	spanMsg, err := json.Marshal(sseMessage{Type: "span", Data: span})
	if err != nil {
		log.Printf("sse marshal span: %v", err)
		return
	}

	t := h.store.GetTrace(span.TraceID)
	traceMsg, err := json.Marshal(sseMessage{Type: "trace", Data: t})
	if err != nil {
		log.Printf("sse marshal trace: %v", err)
		return
	}

	h.mu.Lock()
	for c := range h.clients {
		select {
		case c.ch <- spanMsg:
		default:
		}
		select {
		case c.ch <- traceMsg:
		default:
		}
	}
	h.mu.Unlock()
}
