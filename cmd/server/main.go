package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"relationship-agent-runtime/internal/agent"
	"relationship-agent-runtime/internal/memory"
)

func main() {
	dataDir := getenv("MEMORY_DIR", "data/memory")
	addr := getenv("ADDR", ":8080")
	runtime := agent.NewRuntimeFromEnv(memory.NewJSONStore(dataDir))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveIndex(w)
	})
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

	log.Printf("relationship agent runtime API listening on %s, memory dir=%s", addr, dataDir)
	log.Printf("LLM mode: %s", llmMode())
	log.Printf("This is the API server window. Do not type chat messages here. Use the CLI or POST /chat.")
	log.Printf("Health: http://localhost%s/health", addr)
	log.Printf("Web chat: http://localhost%s/", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && strings.Contains(err.Error(), "bind") {
			log.Printf("Port %s is already in use.", addr)
			log.Printf("Close the old server window, or start this server with another port, for example:")
			log.Printf("$env:ADDR=':8081'; .\\relationship-agent-runtime.exe")
			os.Exit(1)
		}
		log.Fatal(err)
	}
}

func llmMode() string {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		return "off; using deterministic local tools"
	}
	model := getenv("OPENAI_MODEL", "gpt-4o-mini")
	baseURL := getenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	return fmt.Sprintf("on; model=%s base_url=%s", model, baseURL)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json failed: %v", err)
	}
}

func serveIndex(w http.ResponseWriter) {
	candidates := []string{
		filepath.Join("web", "index.html"),
		filepath.Join("..", "..", "web", "index.html"),
	}
	for _, path := range candidates {
		b, err := os.ReadFile(path)
		if err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, "<!doctype html><meta charset=\"utf-8\"><title>Relationship Agent Runtime</title><p>Web UI file not found. Use POST /chat or run the CLI.</p>")
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
