//go:build !cgo
// +build !cgo

package engine

import (
	"context"
	"fmt"
)

type LlamaEngine struct{}

func NewLlamaEngine() *LlamaEngine {
	return &LlamaEngine{}
}

func (e *LlamaEngine) Load(params ModelParams) error {
	return fmt.Errorf("cgo is disabled in this binary. Recompile with CGO_ENABLED=1 to run GGUF weights, or use --mock for testing")
}

func (e *LlamaEngine) ModelInfo() ModelInfo {
	return ModelInfo{
		Name:   "cgo-disabled",
		IsMock: true,
	}
}

func (e *LlamaEngine) Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error) {
	return GenStats{}, fmt.Errorf("cgo is disabled in this binary")
}

func (e *LlamaEngine) Close() error {
	return nil
}
