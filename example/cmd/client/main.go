package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/mickamy/tapbox/example/internal/env"
)

var titles = []string{
	"Buy groceries", "Fix login bug", "Review PR #42",
	"Deploy to staging", "Write unit tests", "Update README",
	"Refactor auth module", "Add dark mode", "Optimize queries",
	"Setup CI pipeline",
}

var bodies = []string{
	"This is urgent", "Low priority", "Blocked by infra team",
	"Needs design review", "Ready for QA", "WIP",
}

func main() {
	target := env.Or("TARGET", "http://localhost:8080")
	intervalStr := env.Or("INTERVAL", "3s")

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("invalid INTERVAL %q: %v", intervalStr, err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	var maxID int64

	log.Printf("Client started: target=%s interval=%s", target, interval)

	for {
		switch rand.IntN(3) { //nolint:gosec // demo client
		case 0: // POST /notes — composite endpoint (gRPC + Connect + HTTP)
			id := createNote(httpClient, target)
			if id > maxID {
				maxID = id
			}
		case 1: // GET /notes
			listNotes(httpClient, target)
		case 2: // GET /notes/{id}
			if maxID > 0 {
				getNote(httpClient, target, rand.Int64N(maxID)+1) //nolint:gosec // demo client
			} else {
				listNotes(httpClient, target)
			}
		}
		time.Sleep(interval)
	}
}

func createNote(client *http.Client, target string) int64 {
	payload, _ := json.Marshal(map[string]string{ //nolint:errchkjson // static map
		"title": titles[rand.IntN(len(titles))], //nolint:gosec // demo client
		"body":  bodies[rand.IntN(len(bodies))], //nolint:gosec // demo client
	})

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target+"/notes", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[REST] POST /notes request: %v", err)
		return 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) //nolint:gosec // URL from config
	if err != nil {
		log.Printf("[REST] POST /notes: %v", err)
		return 0
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Created struct {
			ID int64 `json:"id"`
		} `json:"created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[REST] POST /notes decode: %v", err)
		return 0
	}
	log.Printf("[REST] POST /notes -> %d", resp.StatusCode) //nolint:gosec // status code
	return result.Created.ID
}

func listNotes(client *http.Client, target string) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target+"/notes", nil)
	if err != nil {
		log.Printf("[REST] GET /notes request: %v", err)
		return
	}

	resp, err := client.Do(req) //nolint:gosec // URL from config
	if err != nil {
		log.Printf("[REST] GET /notes: %v", err)
		return
	}
	defer resp.Body.Close()                                //nolint:errcheck
	log.Printf("[REST] GET /notes -> %d", resp.StatusCode) //nolint:gosec // status code
}

func getNote(client *http.Client, target string, id int64) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target+"/notes/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		log.Printf("[REST] GET /notes/%d request: %v", id, err)
		return
	}

	resp, err := client.Do(req) //nolint:gosec // URL from config
	if err != nil {
		log.Printf("[REST] GET /notes/%d: %v", id, err)
		return
	}
	defer resp.Body.Close()                                       //nolint:errcheck
	log.Printf("[REST] GET /notes/%d -> %d", id, resp.StatusCode) //nolint:gosec // status code
}
