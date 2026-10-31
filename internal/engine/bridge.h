#ifndef NOVRAM_BRIDGE_H
#define NOVRAM_BRIDGE_H

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef void* novram_model_handle;

// Callback signature for token streaming to Go runtime
typedef bool (*novram_token_callback)(const char* token, void* user_data);

// Initializes the backend (ggml / llama backend)
int novram_backend_init(void);

// Loads a GGUF model with CPU-optimized settings:
// - n_threads: strict physical core count
// - n_batch: cache-tuned batch size (default 512)
// - n_ctx: context length
// - use_mmap: zero-copy memory mapping
novram_model_handle novram_load_model(
    const char* path,
    int n_threads,
    int n_batch,
    int n_ctx,
    bool use_mmap
);

// Evaluates prompt and streams generated tokens via callback
int novram_generate(
    novram_model_handle handle,
    const char* prompt,
    float temp,
    float top_p,
    int max_tokens,
    novram_token_callback cb,
    void* user_data
);

// Frees context and unmaps memory
void novram_free_model(novram_model_handle handle);

#ifdef __cplusplus
}
#endif

#endif // NOVRAM_BRIDGE_H
