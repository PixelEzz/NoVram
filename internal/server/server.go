package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

// Start binds the HTTP listener and starts serving requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Minute, // Allow long generation streams
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind address %s: %w", addr, err)
	}

	log.Printf("[Server] OpenAI-compatible micro-server running on http://%s", addr)
	log.Printf("[Server] Compatible endpoints: POST /v1/chat/completions (SSE stream supported), GET /v1/models")
	return srv.Serve(ln)
}

// Handler returns the underlying http.Handler for in-memory testing.
func (s *Server) Handler() http.Handler {
	return s.mux
}
