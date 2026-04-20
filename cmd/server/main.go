package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"relationship-agent-runtime/internal/agent"
	"relationship-agent-runtime/internal/memory"
)

func main() {
	dataDir := getenv("MEMORY_DIR", "data/memory")
	addr := getenv("ADDR", ":8080")
	runtime := agent.NewRuntime(memory.NewJSONStore(dataDir))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		var req agent.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, runtime.Chat(req))
	})
	mux.HandleFunc("GET /profile/", func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimPrefix(r.URL.Path, "/profile/")
		profile, err := runtime.Memory.Load(userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, profile)
	})

	log.Printf("relationship agent runtime listening on %s, memory dir=%s", addr, dataDir)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
