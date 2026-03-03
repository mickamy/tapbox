package main

import (
	"bytes"
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
		switch rand.IntN(3) {
		case 0: // POST /notes — composite endpoint (gRPC + Connect + HTTP)
			id := createNote(httpClient, target)
			if id > maxID {
				maxID = id
			}
		case 1: // GET /notes
			listNotes(httpClient, target)
		case 2: // GET /notes/{id}
			if maxID > 0 {
				getNote(httpClient, target, rand.Int64N(maxID)+1)
			} else {
				listNotes(httpClient, target)
			}
		}
		time.Sleep(interval)
	}
}

func createNote(client *http.Client, target string) int64 {
	payload, _ := json.Marshal(map[string]string{
		"title": titles[rand.IntN(len(titles))],
		"body":  bodies[rand.IntN(len(bodies))],
	})

	resp, err := client.Post(target+"/notes", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[REST] POST /notes: %v", err)
		return 0
	}
	defer resp.Body.Close()

	var result struct {
		Created struct {
			ID int64 `json:"id"`
		} `json:"created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[REST] POST /notes decode: %v", err)
		return 0
	}
	log.Printf("[REST] POST /notes -> %d", resp.StatusCode)
	return result.Created.ID
}

func listNotes(client *http.Client, target string) {
	resp, err := client.Get(target + "/notes")
	if err != nil {
		log.Printf("[REST] GET /notes: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[REST] GET /notes -> %d", resp.StatusCode)
}

func getNote(client *http.Client, target string, id int64) {
	resp, err := client.Get(target + "/notes/" + strconv.FormatInt(id, 10))
	if err != nil {
		log.Printf("[REST] GET /notes/%d: %v", id, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[REST] GET /notes/%d -> %d", id, resp.StatusCode)
}
