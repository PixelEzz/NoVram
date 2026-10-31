package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockEngine is an in-memory test engine for integration testing, CI, and running without heavy model weights.
type MockEngine struct {
	mu     sync.Mutex
	params ModelParams
	loaded bool
}

// NewMockEngine creates a new simulated engine.
func NewMockEngine() *MockEngine {
	return &MockEngine{}
}

func (m *MockEngine) Load(params ModelParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.params = params
	m.loaded = true
	return nil
}

func (m *MockEngine) ModelInfo() ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := "novram-mock-cpu-model"
	if m.params.ModelPath != "" {
		name = m.params.ModelPath
	}

	return ModelInfo{
		Name:        name,
		Path:        m.params.ModelPath,
		ContextSize: m.params.ContextSize,
		BatchSize:   m.params.BatchSize,
		Threads:     m.params.Threads,
		UseMmap:     m.params.UseMmap,
		IsMock:      true,
	}
}

func (m *MockEngine) Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error) {
	m.mu.Lock()
	if !m.loaded {
		m.mu.Unlock()
		return GenStats{}, fmt.Errorf("model is not loaded")
	}
	m.mu.Unlock()

	startTime := time.Now()
	// Simulate prompt eval latency (~30ms)
	time.Sleep(30 * time.Millisecond)

	var firstTokenTime time.Time

	// Construct simulated response
	var responseTokens []string
	promptLower := strings.ToLower(params.Prompt)

	if strings.Contains(promptLower, "hello") || strings.Contains(promptLower, "hi") {
		responseTokens = strings.Split("Hello! I am running entirely on your CPU via novram with zero VRAM required. How can I assist your coding today?", " ")
	} else if strings.Contains(promptLower, "who are you") {
		responseTokens = strings.Split("I am an open-source language model powered by novram, an ultra-fast CPU-only inference engine optimized for cache locality and physical core affinity.", " ")
	} else {
		responseTokens = strings.Split("Novram CPU inference engine active. Physical core affinity enabled. Zero-copy mmap active. Tokens streaming with minimal latency directly from RAM.", " ")
	}

	tokenCount := 0
	for i, word := range responseTokens {
		select {
		case <-ctx.Done():
			return CalculateStats(15, tokenCount, startTime, firstTokenTime, time.Now()), ctx.Err()
		default:
		}

		if i == 0 {
			firstTokenTime = time.Now()
		}

		tok := word
		if i > 0 {
			tok = " " + word
		}

		cont := onToken(tok)
		tokenCount++
		if !cont {
			break
		}

		// Simulate ~32 tokens per second (approx 30ms per token)
		time.Sleep(28 * time.Millisecond)
	}

	endTime := time.Now()
	promptTokens := len(strings.Fields(params.Prompt))
	if promptTokens == 0 {
		promptTokens = 10
	}

	return CalculateStats(promptTokens, tokenCount, startTime, firstTokenTime, endTime), nil
}

func (m *MockEngine) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loaded = false
	return nil
}
