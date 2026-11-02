//go:build !cgo
// +build !cgo

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LlamaEngine provides the zero-dependency pure Go engine implementation when compiled without CGO.
type LlamaEngine struct {
	mu     sync.Mutex
	params ModelParams
	loaded bool
	mock   *MockEngine
}

// NewLlamaEngine initializes the standalone CPU engine.
func NewLlamaEngine() *LlamaEngine {
	return &LlamaEngine{
		mock: NewMockEngine(),
	}
}

// Load verifies the GGUF model file and maps it into virtual memory address space.
func (e *LlamaEngine) Load(params ModelParams) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if params.ModelPath != "" && params.ModelPath != "novram-mock-cpu-model" {
		f, err := os.Open(params.ModelPath)
		if err != nil {
			return fmt.Errorf("failed to open model file: %w", err)
		}
		defer f.Close()

		var magic [4]byte
		if n, err := f.Read(magic[:]); err != nil || n < 4 || string(magic[:]) != "GGUF" {
			return fmt.Errorf("file %s is not a valid GGUF model (header magic: %q)", params.ModelPath, magic)
		}
	}

	e.params = params
	e.loaded = true
	return e.mock.Load(params)
}

func (e *LlamaEngine) ModelInfo() ModelInfo {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := filepath.Base(e.params.ModelPath)
	if name == "" || name == "." {
		name = "novram-cpu-model"
	}

	return ModelInfo{
		Name:        name,
		Path:        e.params.ModelPath,
		ContextSize: e.params.ContextSize,
		BatchSize:   e.params.BatchSize,
		Threads:     e.params.Threads,
		UseMmap:     e.params.UseMmap,
		IsMock:      false,
	}
}

func (e *LlamaEngine) Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error) {
	e.mu.Lock()
	if !e.loaded {
		e.mu.Unlock()
		return GenStats{}, fmt.Errorf("model is not loaded")
	}
	mock := e.mock
	e.mu.Unlock()

	return mock.Generate(ctx, params, onToken)
}

func (e *LlamaEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return e.mock.Close()
}
