package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/novram-project/novram/internal/engine"
	"github.com/novram-project/novram/internal/server"
)

func setupTestServer() *server.Server {
	eng := engine.NewMockEngine()
	_ = eng.Load(engine.DefaultModelParams("mock-test-model.gguf"))
	return server.NewServer(eng, "127.0.0.1", 8080)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp server.HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Threads <= 0 {
		t.Errorf("Expected positive threads count, got %d", resp.Threads)
	}
}

func TestModelsEndpoint(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp server.ModelListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to parse models response: %v", err)
	}

	if len(resp.Data) == 0 {
		t.Fatalf("Expected at least one model in list")
	}
	if resp.Data[0].OwnedBy != "novram" {
		t.Errorf("Expected owned_by 'novram', got '%s'", resp.Data[0].OwnedBy)
	}
}

func TestChatCompletionsNonStream(t *testing.T) {
	srv := setupTestServer()

	chatReq := server.ChatCompletionRequest{
		Model: "mock-test-model.gguf",
		Messages: []server.ChatMessage{
			{Role: "user", Content: "Hello novram"},
		},
		Stream: false,
	}

	body, _ := json.Marshal(chatReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp server.ChatCompletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatalf("Expected non-empty choices")
	}
	if resp.Choices[0].Message.Content == "" {
		t.Errorf("Expected non-empty message content")
	}
	if resp.Usage.CompletionTokens <= 0 {
		t.Errorf("Expected positive completion tokens, got %d", resp.Usage.CompletionTokens)
	}
}

func TestChatCompletionsStream(t *testing.T) {
	srv := setupTestServer()

	chatReq := server.ChatCompletionRequest{
		Model: "mock-test-model.gguf",
		Messages: []server.ChatMessage{
			{Role: "user", Content: "Who are you?"},
		},
		Stream: true,
	}

	body, _ := json.Marshal(chatReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected text/event-stream content type, got: %s", contentType)
	}

	out := w.Body.String()
	if !strings.Contains(out, "data: [DONE]") {
		t.Errorf("Expected SSE stream to terminate with 'data: [DONE]', got: %s", out)
	}
	if !strings.Contains(out, "chat.completion.chunk") {
		t.Errorf("Expected chunks with object 'chat.completion.chunk'")
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := setupTestServer()
	req := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected status 204 No Content for OPTIONS, got %d", w.Code)
	}

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got: %s", origin)
	}
}
