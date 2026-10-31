package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/novram-project/novram/internal/cpu"
	"github.com/novram-project/novram/internal/engine"
)

// Server handles HTTP API requests.
type Server struct {
	engine engine.Engine
	mux    *http.ServeMux
	host   string
	port   int
}

// NewServer initializes the server with engine and routing.
func NewServer(eng engine.Engine, host string, port int) *Server {
	s := &Server{
		engine: eng,
		mux:    http.NewServeMux(),
		host:   host,
		port:   port,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/v1/chat/completions", s.cors(s.handleChatCompletions))
	s.mux.HandleFunc("/v1/models", s.cors(s.handleModels))
	s.mux.HandleFunc("/health", s.cors(s.handleHealth))
	s.mux.HandleFunc("/v1/health", s.cors(s.handleHealth))
}

func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	info := s.engine.ModelInfo()
	modelID := info.Name
	if modelID == "" {
		modelID = "novram-cpu-model"
	}

	resp := ModelListResponse{
		Object: "list",
		Data: []ModelItem{
			{
				ID:      modelID,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "novram",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	info := cpu.Detect()
	modelInfo := s.engine.ModelInfo()

	resp := HealthResponse{
		Status:        "ok",
		Model:         modelInfo.Name,
		Threads:       info.PhysicalCores,
		VectorSupport: info.VectorSummary(),
		L3CacheKB:     info.L3CacheKB,
		FreeRAMGB:     float64(info.FreeRAMBytes) / (1024 * 1024 * 1024),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, "messages array cannot be empty", http.StatusBadRequest)
		return
	}

	// Format prompt using standard ChatML template
	prompt := formatChatML(req.Messages)
	genParams := engine.GenParams{
		Prompt:        prompt,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		MaxTokens:     req.MaxTokens,
		StopSequences: req.Stop,
	}

	reqID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	modelName := s.engine.ModelInfo().Name
	if modelName == "" {
		modelName = req.Model
	}

	if req.Stream {
		s.streamResponse(w, r.Context(), reqID, modelName, genParams)
	} else {
		s.nonStreamResponse(w, r.Context(), reqID, modelName, genParams)
	}
}

func (s *Server) streamResponse(w http.ResponseWriter, ctx context.Context, reqID, modelName string, genParams engine.GenParams) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported by client connection", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// 1. Initial role chunk
	initialChunk := ChatCompletionChunk{
		ID:      reqID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []ChunkChoice{
			{
				Index: 0,
				Delta: ChunkDelta{
					Role: "assistant",
				},
				FinishReason: nil,
			},
		},
	}
	sendSSEChunk(w, flusher, initialChunk)

	// 2. Stream token chunks
	_, err := s.engine.Generate(ctx, genParams, func(token string) bool {
		chunk := ChatCompletionChunk{
			ID:      reqID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []ChunkChoice{
				{
					Index: 0,
					Delta: ChunkDelta{
						Content: token,
					},
					FinishReason: nil,
				},
			},
		}
		sendSSEChunk(w, flusher, chunk)
		return true
	})

	if err != nil && ctx.Err() == nil {
		// Log generation error if client didn't disconnect
	}

	// 3. Final stop chunk
	stopReason := "stop"
	finalChunk := ChatCompletionChunk{
		ID:      reqID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []ChunkChoice{
			{
				Index:        0,
				Delta:        ChunkDelta{},
				FinishReason: &stopReason,
			},
		},
	}
	sendSSEChunk(w, flusher, finalChunk)

	// 4. SSE sentinel
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) nonStreamResponse(w http.ResponseWriter, ctx context.Context, reqID, modelName string, genParams engine.GenParams) {
	var sb strings.Builder

	stats, err := s.engine.Generate(ctx, genParams, func(token string) bool {
		sb.WriteString(token)
		return true
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("generation error: %v", err), http.StatusInternalServerError)
		return
	}

	resp := ChatCompletionResponse{
		ID:      reqID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: sb.String(),
				},
				FinishReason: "stop",
			},
		},
		Usage: UsageStats{
			PromptTokens:     stats.PromptTokens,
			CompletionTokens: stats.CompletionTokens,
			TotalTokens:      stats.TotalTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func sendSSEChunk(w http.ResponseWriter, flusher http.Flusher, chunk ChatCompletionChunk) {
	data, err := json.Marshal(chunk)
	if err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// formatChatML wraps chat messages in standard ChatML prompt delimiters.
func formatChatML(messages []ChatMessage) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString("<|im_start|>")
		sb.WriteString(m.Role)
		sb.WriteString("\n")
		sb.WriteString(m.Content)
		sb.WriteString("<|im_end|>\n")
	}
	sb.WriteString("<|im_start|>assistant\n")
	return sb.String()
}
