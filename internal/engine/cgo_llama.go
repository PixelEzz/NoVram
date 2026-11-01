//go:build cgo
// +build cgo

package engine

/*
#cgo CFLAGS: -O3 -I.
#cgo CXXFLAGS: -O3 -std=c++17 -I.
#include "bridge.h"

// Forward declaration of Go export callback
extern bool goTokenCallback(char* token, void* userData);

static inline int call_novram_generate(novram_model_handle handle, const char* prompt, float temp, float top_p, int max_tokens, void* userData) {
    return novram_generate(handle, prompt, temp, top_p, max_tokens, (novram_token_callback)goTokenCallback, userData);
}
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"
)

type LlamaEngine struct {
	mu     sync.Mutex
	handle C.novram_model_handle
	params ModelParams
	loaded bool
}

// NewLlamaEngine creates an instance of the llama.cpp Cgo engine.
func NewLlamaEngine() *LlamaEngine {
	C.novram_backend_init()
	return &LlamaEngine{}
}

func (e *LlamaEngine) Load(params ModelParams) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cPath := C.CString(params.ModelPath)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.novram_load_model(
		cPath,
		C.int(params.Threads),
		C.int(params.BatchSize),
		C.int(params.ContextSize),
		C.bool(params.UseMmap),
	)

	if handle == nil {
		return fmt.Errorf("failed to load model from path %s: check file existence or build with llama.cpp support", params.ModelPath)
	}

	e.handle = handle
	e.params = params
	e.loaded = true
	return nil
}

func (e *LlamaEngine) ModelInfo() ModelInfo {
	e.mu.Lock()
	defer e.mu.Unlock()

	return ModelInfo{
		Name:        e.params.ModelPath,
		Path:        e.params.ModelPath,
		ContextSize: e.params.ContextSize,
		BatchSize:   e.params.BatchSize,
		Threads:     e.params.Threads,
		UseMmap:     e.params.UseMmap,
		IsMock:      false,
	}
}

type tokenStreamSession struct {
	ctx        context.Context
	onToken    func(token string) bool
	firstToken time.Time
	count      int
}

var (
	sessionsMu sync.Mutex
	sessions   = make(map[uintptr]*tokenStreamSession)
	nextSessID uintptr
)

//export goTokenCallback
func goTokenCallback(token *C.char, userData unsafe.Pointer) C.bool {
	sessID := uintptr(userData)
	sessionsMu.Lock()
	sess, ok := sessions[sessID]
	sessionsMu.Unlock()

	if !ok || sess == nil {
		return C.bool(false)
	}

	select {
	case <-sess.ctx.Done():
		return C.bool(false)
	default:
	}

	if sess.firstToken.IsZero() {
		sess.firstToken = time.Now()
	}

	goStr := C.GoString(token)
	sess.count++
	cont := sess.onToken(goStr)
	return C.bool(cont)
}

func (e *LlamaEngine) Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error) {
	e.mu.Lock()
	if !e.loaded || e.handle == nil {
		e.mu.Unlock()
		return GenStats{}, fmt.Errorf("engine model not loaded")
	}
	e.mu.Unlock()

	sess := &tokenStreamSession{
		ctx:     ctx,
		onToken: onToken,
	}

	sessionsMu.Lock()
	nextSessID++
	id := nextSessID
	sessions[id] = sess
	sessionsMu.Unlock()

	defer func() {
		sessionsMu.Lock()
		delete(sessions, id)
		sessionsMu.Unlock()
	}()

	startTime := time.Now()
	cPrompt := C.CString(params.Prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	maxTokens := params.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	ret := C.call_novram_generate(
		e.handle,
		cPrompt,
		C.float(params.Temperature),
		C.float(params.TopP),
		C.int(maxTokens),
		unsafe.Pointer(id),
	)

	endTime := time.Now()
	if ret < 0 {
		return GenStats{}, fmt.Errorf("inference failed with code: %d", int(ret))
	}

	return CalculateStats(len(params.Prompt)/4, sess.count, startTime, sess.firstToken, endTime), nil
}

func (e *LlamaEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.handle != nil {
		C.novram_free_model(e.handle)
		e.handle = nil
		e.loaded = false
	}
	return nil
}
