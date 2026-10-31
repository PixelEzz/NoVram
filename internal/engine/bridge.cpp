#include "bridge.h"
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#ifdef NOVRAM_HAS_LLAMA
#include "llama.h"

struct NovramContext {
    llama_model* model = nullptr;
    llama_context* ctx = nullptr;
    int n_threads = 4;
    int n_batch = 512;
    int n_ctx = 4096;
};

int novram_backend_init(void) {
    llama_backend_init();
    return 0;
}

novram_model_handle novram_load_model(
    const char* path,
    int n_threads,
    int n_batch,
    int n_ctx,
    bool use_mmap
) {
    if (!path) return nullptr;

    llama_model_params mparams = llama_model_default_params();
    mparams.use_mmap = use_mmap;

    llama_model* model = llama_load_model_from_file(path, mparams);
    if (!model) return nullptr;

    llama_context_params cparams = llama_context_default_params();
    cparams.n_threads = n_threads;
    cparams.n_threads_batch = n_threads;
    cparams.n_batch = n_batch;
    cparams.n_ctx = n_ctx;

    llama_context* ctx = llama_new_context_with_model(model, cparams);
    if (!ctx) {
        llama_free_model(model);
        return nullptr;
    }

    NovramContext* nc = new NovramContext();
    nc->model = model;
    nc->ctx = ctx;
    nc->n_threads = n_threads;
    nc->n_batch = n_batch;
    nc->n_ctx = n_ctx;
    return (novram_model_handle)nc;
}

int novram_generate(
    novram_model_handle handle,
    const char* prompt,
    float temp,
    float top_p,
    int max_tokens,
    novram_token_callback cb,
    void* user_data
) {
    NovramContext* nc = (NovramContext*)handle;
    if (!nc || !nc->ctx || !nc->model || !prompt) return -1;

    // Tokenize prompt
    const int n_prompt_max = strlen(prompt) + 128;
    std::vector<llama_token> tokens(n_prompt_max);
    int n_tokens = llama_tokenize(nc->model, prompt, strlen(prompt), tokens.data(), tokens.size(), true, false);
    if (n_tokens < 0) return -2;
    tokens.resize(n_tokens);

    // Evaluate prompt tokens in cache-friendly batches
    llama_batch batch = llama_batch_init(nc->n_batch, 0, 1);
    for (size_t i = 0; i < tokens.size(); i += nc->n_batch) {
        size_t n_eval = (tokens.size() - i > (size_t)nc->n_batch) ? (size_t)nc->n_batch : (tokens.size() - i);
        batch.n_tokens = 0;
        for (size_t j = 0; j < n_eval; j++) {
            llama_batch_add(batch, tokens[i + j], i + j, {0}, (j == n_eval - 1));
        }
        if (llama_decode(nc->ctx, batch) != 0) {
            llama_batch_free(batch);
            return -3;
        }
    }

    // Generation loop
    int n_gen = 0;
    while (n_gen < max_tokens) {
        // Sample next token
        llama_token new_token_id = llama_sampler_sample(nullptr, nc->ctx, -1);
        if (llama_token_is_eog(nc->model, new_token_id)) {
            break;
        }

        // Convert token to piece
        char buf[128];
        int n_piece = llama_token_to_piece(nc->model, new_token_id, buf, sizeof(buf), 0, true);
        if (n_piece > 0) {
            buf[n_piece] = '\0';
            if (cb) {
                if (!cb(buf, user_data)) {
                    break;
                }
            }
        }

        // Decode new token
        batch.n_tokens = 0;
        llama_batch_add(batch, new_token_id, tokens.size() + n_gen, {0}, true);
        if (llama_decode(nc->ctx, batch) != 0) {
            break;
        }
        n_gen++;
    }

    llama_batch_free(batch);
    return n_gen;
}

void novram_free_model(novram_model_handle handle) {
    NovramContext* nc = (NovramContext*)handle;
    if (nc) {
        if (nc->ctx) llama_free(nc->ctx);
        if (nc->model) llama_free_model(nc->model);
        delete nc;
    }
}

#else

// Stubs when llama.cpp is not linked directly at compilation
int novram_backend_init(void) {
    return 0;
}

novram_model_handle novram_load_model(
    const char* path,
    int n_threads,
    int n_batch,
    int n_ctx,
    bool use_mmap
) {
    return nullptr;
}

int novram_generate(
    novram_model_handle handle,
    const char* prompt,
    float temp,
    float top_p,
    int max_tokens,
    novram_token_callback cb,
    void* user_data
) {
    return -1;
}

void novram_free_model(novram_model_handle handle) {
}

#endif
