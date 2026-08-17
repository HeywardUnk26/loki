package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Stream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type PushRequest struct {
	Streams []Stream `json:"streams"`
}

type LabelValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

var (
	streams []Stream
	mu      sync.RWMutex
)

func main() {
	http.HandleFunc("/loki/api/v1/push", handlePush)
	http.HandleFunc("/loki/api/v1/label/", handleLabelValues)
	log.Println("Starting mock Loki server on :3100...")
	log.Fatal(http.ListenAndServe(":3100", nil))
}

func handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mu.Lock()
	streams = append(streams, req.Streams...)
	mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func handleLabelValues(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	// Expected path format: /loki/api/v1/label/{name}/values
	parts := strings.Split(path, "/")
	if len(parts) < 6 || parts[len(parts)-1] != "values" || parts[len(parts)-3] != "label" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	labelName := parts[len(parts)-2]

	q := r.URL.Query()
	var startNano, endNano int64
	now := time.Now().UnixNano()

	// Default to last 6 hours if start/end not provided
	startStr := q.Get("start")
	if startStr != "" {
		if s, err := parseTime(startStr); err == nil {
			startNano = s
		} else {
			http.Error(w, "Invalid start time", http.StatusBadRequest)
			return
		}
	} else {
		startNano = now - int64(6*time.Hour)
	}

	endStr := q.Get("end")
	if endStr != "" {
		if e, err := parseTime(endStr); err == nil {
			endNano = e
		} else {
			http.Error(w, "Invalid end time", http.StatusBadRequest)
			return
		}
	} else {
		endNano = now
	}

	matchers := q["match[]"]
	parsedMatchers := make(map[string]string)
	for _, m := range matchers {
		// Simple parser for matchers like {env="production", app="payment"}
		m = strings.Trim(m, "{}")
		parts := strings.Split(m, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// Support = and != (simple mock)
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				k := strings.TrimSpace(kv[0])
				v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				parsedMatchers[k] = v
			}
		}
	}

	valueSet := make(map[string]bool)
	mu.RLock()
	for _, stream := range streams {
		// Check matchers
		matched := true
		for k, v := range parsedMatchers {
			if streamVal, ok := stream.Stream[k]; !ok || streamVal != v {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}

		// Check if stream has label
		val, ok := stream.Stream[labelName]
		if !ok {
			continue
		}

		// Check time range
		hasValidTime := false
		for _, valPair := range stream.Values {
			if len(valPair) > 0 {
				tNano, err := strconv.ParseInt(valPair[0], 10, 64)
				if err == nil {
					if tNano >= startNano && tNano <= endNano {
						hasValidTime = true
						break
					}
				}
			}
		}

		if hasValidTime {
			valueSet[val] = true
			// Also check if the label value itself is part of the stream
		}
	}
	mu.RUnlock()

	values := make([]string, 0, len(valueSet))
	for v := range valueSet {
		values = append(values, v)
	}

	resp := LabelValuesResponse{
		Status: "success",
		Data:   values,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func parseTime(s string) (int64, error) {
	// Try parsing as unix timestamp (seconds or nanoseconds)
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		// If it's small, it's probably seconds, convert to nanoseconds
		if val < 10000000000 {
			return val * 1000000000, nil
		}
		return val, nil
	}
	// Try parsing RFC3339
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t.UnixNano(), nil
	}
	return 0, fmt.Errorf("invalid time format")
}
