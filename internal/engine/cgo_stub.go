//go:build !cgo
// +build !cgo

package engine

import (
	"context"
	"sync"
)

// LlamaEngine wraps the native CPU BackendEngine for zero-dependency non-CGO builds.
type LlamaEngine struct {
	mu      sync.Mutex
	backend *BackendEngine
	mock    *MockEngine
	isMock  bool
}

// NewLlamaEngine initializes the standalone CPU engine using the native BackendEngine supervisor.
func NewLlamaEngine() *LlamaEngine {
	return &LlamaEngine{
		backend: NewBackendEngine(),
		mock:    NewMockEngine(),
	}
}

// Load verifies the GGUF model and starts the native backend runner.
func (e *LlamaEngine) Load(params ModelParams) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if params.ModelPath == "novram-mock-cpu-model" {
		e.isMock = true
		return e.mock.Load(params)
	}

	e.isMock = false
	return e.backend.Load(params)
}

// ModelInfo returns runtime metadata about the model.
func (e *LlamaEngine) ModelInfo() ModelInfo {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isMock {
		return e.mock.ModelInfo()
	}
	return e.backend.ModelInfo()
}

// Generate streams real neural network tokens from the CPU runner.
func (e *LlamaEngine) Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error) {
	e.mu.Lock()
	isMock := e.isMock
	e.mu.Unlock()

	if isMock {
		return e.mock.Generate(ctx, params, onToken)
	}
	return e.backend.Generate(ctx, params, onToken)
}

// Close terminates the backend process and frees resources.
func (e *LlamaEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isMock {
		return e.mock.Close()
	}
	return e.backend.Close()
}
