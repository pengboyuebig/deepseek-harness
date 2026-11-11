// Package api is the HTTP surface over the agent: health, composed config, and
// a server-sent event stream, mirroring packages/api.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"git.glkj.example.com/glkj/harness/go/internal/config"
	"git.glkj.example.com/glkj/harness/go/internal/web"
)

// Server wires the HTTP handlers.
type Server struct {
	Config  config.Config
	Compose func() (string, error)
	RepoDir string
}

// Handler returns the root mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/config", s.dumpConfig)
	mux.HandleFunc("/api/events", s.events)
	mux.Handle("/", web.Handler(s.RepoDir))
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"profile": s.Config.Profile,
		"model":   s.Config.Model,
	})
}

func (s *Server) dumpConfig(w http.ResponseWriter, _ *http.Request) {
	if s.Compose == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no profile composed"})
		return
	}
	dump, err := s.Compose()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, dump)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Addr formats a listen address from host and port.
func Addr(host string, port int) string {
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", host, port)
}
