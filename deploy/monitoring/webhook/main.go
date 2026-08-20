package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type eventState struct {
	mu       sync.Mutex
	Received int64     `json:"received"`
	Firing   int64     `json:"firing"`
	Resolved int64     `json:"resolved"`
	LastAt   time.Time `json:"lastAt,omitempty"`
}

var webhookClient = &http.Client{Timeout: 10 * time.Second}

func main() {
	state := &eventState{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, _ *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	})
	mux.HandleFunc("POST /alerts", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read alert", http.StatusBadRequest)
			return
		}
		var payload struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || (payload.Status != "firing" && payload.Status != "resolved") {
			http.Error(w, "invalid alert payload", http.StatusBadRequest)
			return
		}
		state.mu.Lock()
		state.Received++
		state.LastAt = time.Now().UTC()
		if payload.Status == "firing" {
			state.Firing++
		} else {
			state.Resolved++
		}
		state.mu.Unlock()
		if err := forward(body); err != nil {
			log.Printf("forward alert: %v", err)
			http.Error(w, "forward alert", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func forward(body []byte) error {
	url := os.Getenv("MONITORING_WEBHOOK_FORWARD_URL")
	if url == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("MONITORING_WEBHOOK_FORWARD_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &forwardStatusError{status: resp.Status}
	}
	return nil
}

type forwardStatusError struct{ status string }

func (err *forwardStatusError) Error() string { return "unexpected status " + err.status }
