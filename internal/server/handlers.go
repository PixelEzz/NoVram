package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/novram-project/novram/internal/cpu"
	"github.com/novram-project/novram/internal/engine"
	"github.com/novram-project/novram/internal/interceptor"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

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
		w.Header().Set("Access-Control-Expose-Headers", "X-Novram-Interception, X-Novram-Recommended-Model, X-Novram-Recommended-Quant, X-Novram-Safety-Status, X-Novram-Recommendation-Note")
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

	data := []ModelItem{
		{
			ID:      modelID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "novram",
		},
	}

	// Add catalog models for IDE dropdowns
	for _, m := range interceptor.Catalog {
		if m.ID != modelID {
			data = append(data, ModelItem{
				ID:      m.ID,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "novram-catalog",
			})
		}
	}

	resp := ModelListResponse{
		Object: "list",
		Data:   data,
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

	// Interceptor Optimization Layer:
	// Evaluates the requested model against host hardware and injects optimization telemetry
	hw := cpu.Detect()
	decision := interceptor.InterceptAndOptimize(hw, req.Model, req.MaxTokens)

	w.Header().Set("X-Novram-Interception", "optimized")
	w.Header().Set("X-Novram-Host-CPU", hw.ModelName)
	w.Header().Set("X-Novram-Requested-Model", req.Model)
	w.Header().Set("X-Novram-Recommended-Model", decision.SelectedModel)
	w.Header().Set("X-Novram-Recommended-Quant", decision.RecommendedQuant)
	w.Header().Set("X-Novram-Safety-Status", decision.SafetyStatus)
	w.Header().Set("X-Novram-Recommendation-Note", decision.RecommendationNote)

	log.Printf("[Interceptor] Intercepted '%s' -> Recommended CPU Model: %s (%s) [%s | %.1f tok/s est]",
		req.Model, decision.SelectedModel, decision.RecommendedQuant, decision.SafetyStatus, decision.EstTokensPerSec)

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

	created := time.Now().Unix()

	// 1. Initial role chunk
	initialChunk := ChatCompletionChunk{
		ID:      reqID,
		Object:  "chat.completion.chunk",
		Created: created,
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

	// 2. Stream token chunks with pooled buffer
	_, err := s.engine.Generate(ctx, genParams, func(token string) bool {
		sendSSEToken(w, flusher, reqID, modelName, created, token)
		return true
	})

	if err != nil && ctx.Err() == nil {
		// Client remains connected but generation failed
	}

	// 3. Final stop chunk
	stopReason := "stop"
	finalChunk := ChatCompletionChunk{
		ID:      reqID,
		Object:  "chat.completion.chunk",
		Created: created,
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

func sendSSEToken(w http.ResponseWriter, flusher http.Flusher, reqID, modelName string, created int64, token string) {
	escapedToken, _ := json.Marshal(token)
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	buf.WriteString("data: {\"id\":")
	buf.WriteString(strconv.Quote(reqID))
	buf.WriteString(",\"object\":\"chat.completion.chunk\",\"created\":")
	buf.WriteString(strconv.FormatInt(created, 10))
	buf.WriteString(",\"model\":")
	buf.WriteString(strconv.Quote(modelName))
	buf.WriteString(",\"choices\":[{\"index\":0,\"delta\":{\"content\":")
	buf.Write(escapedToken)
	buf.WriteString("},\"finish_reason\":null}]}\n\n")

	w.Write(buf.Bytes())
	flusher.Flush()
}

func sendSSEChunk(w http.ResponseWriter, flusher http.Flusher, chunk ChatCompletionChunk) {
	data, err := json.Marshal(chunk)
	if err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// formatChatML wraps chat messages in standard ChatML prompt delimiters with pre-allocated buffer.
func formatChatML(messages []ChatMessage) string {
	totalLen := 32
	for _, m := range messages {
		totalLen += len(m.Role) + len(m.Content) + 24
	}

	var sb strings.Builder
	sb.Grow(totalLen)
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
