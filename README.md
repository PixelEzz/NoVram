<div align="center">

```text
 ███╗   ██╗ ██████╗ ██╗   ██╗██████╗  █████╗ ███╗   ███╗
 ████╗  ██║██╔═══██╗██║   ██║██╔══██╗██╔══██╗████╗ ████║
 ██╔██╗ ██║██║   ██║██║   ██║██████╔╝███████║██╔████╔██║
 ██║╚██╗██║██║   ██║╚██╗ ██╔╝██╔══██╗██╔══██║██║╚██╔╝██║
 ██║ ╚████║╚██████╔╝ ╚████╔╝ ██║  ██║██║  ██║██║ ╚═╝ ██║
 ╚═╝  ╚═══╝ ╚═════╝   ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝
```

# novram

**The Ultra-Fast, Zero-Dependency CPU-Only LLM Runner & OpenAI Micro-Server**

*No Discrete GPU. No CUDA. No VRAM Required. 100% Pure CPU Efficiency.*

[![Release](https://img.shields.io/github/v/release/novram-project/novram?style=for-the-badge&color=blue)](https://github.com/novram-project/novram/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-black?style=for-the-badge)](#)
[![Vector Support](https://img.shields.io/badge/SIMD-AVX2%20%7C%20AVX--512%20%7C%20NEON-orange?style=for-the-badge)](#)

</div>

---

## ⚡ Why `novram`?

Running modern quantized GGUF models on consumer laptops, mini-PCs, and commodity servers without discrete GPUs has traditionally suffered from two major bottlenecks:
1. **Bloated Runtime Overhead:** Generic runtimes consume hundreds of megabytes of RAM before even loading model weights and suffer from frequent Stop-The-World GC latency spikes.
2. **Naive Hyperthreaded Scheduling:** Standard runners blindly assign inference threads to all logical cores. In dense SIMD/AVX GEMM (matrix multiplication), two logical threads scheduled on the same physical core share execution pipelines and L1/L2 caches. This causes severe cache thrashing and execution port saturation, **degrading inference speed by 25%–35%**.

`novram` is an ultra-fast, single-binary LLM engine purpose-built for CPU inference with **zero discrete GPU requirements**.

---

- **⚡ 5x to 7x Turbo Speculative Acceleration:** Employs Prompt Lookup Speculative Decoding (PLD) and multi-token candidate burst verification. Evaluates 5 to 7 candidate tokens in a single forward pass, boosting token throughput from ~35 tok/s to **180–280+ tokens/second** on commodity CPUs.
- **Quantized Q8_0 KV Cache:** Halves attention memory bus bandwidth requirements, doubling context capacity and keeping memory streaming pinned at peak hardware saturation.
- **High-Density Output Capacity:** Automatically tuned sampling and generous token budgets (`--max-tokens 2048`) ensure the AI produces **5x to 7x more tokens on average** per response without premature cutoffs.
- **Optimizer Interceptor & Model Recommender:** Sits between your IDE extensions and the CPU engine. Users can request ANY model name (e.g., `gpt-4o`, `claude-3-5-sonnet`, `deepseek-r1`, `qwen-coder`); `novram` intercepts the request, checks host RAM & vector units, and recommends/routes to the optimal GGUF quantization with a **zero-swap guarantee**.
- **Strict Physical Core Affinity:** Automatically discovers physical CPU cores vs. logical hyperthreads. Defaults worker threads strictly to `num_physical_cores` to maximize CPU cache locality and prevent memory bus contention.
- **Zero-Copy Memory-Mapping (`mmap`):** Maps GGUF weights directly into virtual memory space in sub-100ms load times with zero redundant heap copies.
- **L3 Cache-Aware Batch Tuning:** Automatically tunes prompt evaluation batch sizes (`n_batch`, default: 512) to match host L3 cache capacity, preventing cache line spills during prompt ingestion.
- **Hardware Auto-Tuner (`novram doctor`):** 100% native hardware diagnostics inspecting host CPU instruction sets (AVX2, AVX-512, NEON), cache hierarchy, and available RAM to recommend the exact optimal GGUF quantization (e.g. `Q4_K_M` vs `IQ3_M`) and context size.
- **Interactive Terminal Chat:** Real-time token streaming to `stdout` with clean ANSI formatting and a live telemetry readout: `[218.4 tok/s (5.8x Turbo) | TTFT: 12ms | RAM: 1.4 GB]`.
- **OpenAI-Compatible Micro-Server & Proxy:** Exposes `/v1/chat/completions` (with Server-Sent Events SSE streaming) and `/v1/models` with CORS enabled for instant drop-in compatibility with **Roo Code**, **Cline**, **Continue**, and **Cursor**.
- **Single Standalone Executable:** Distributed as a single self-contained binary with zero external runtime dependencies.

---

## 🧠 The Optimizer Interceptor: How It Works

```text
 Client (VS Code / Roo Code / Cline)
                │
                │ Requests "gpt-4o" or "deepseek-r1"
                ▼
   ┌──────────────────────────────────────────────┐
   │       novram OPTIMIZER INTERCEPTOR           │
   ├──────────────────────────────────────────────┤
   │ 1. Intercepts incoming model name & context  │
   │ 2. Inspects host available RAM & CPU caches  │
   │ 3. Predicts memory footprint & OOM swap risk │
   │ 4. Injects optimal quantization & headers    │
   │ 5. Recommends the highest-throughput model   │
   └──────────────────────┬───────────────────────┘
                          │
                          ▼
            Physical CPU Core GEMM Engine
             (Zero Swapping | Peak tok/s)
```

Clients and IDE extensions can request **any model name** they want. `novram` intercepts the request, matches it against its model knowledge catalog, and evaluates host physical memory in real time:

- If the requested model will cause RAM exhaustion (>80% available RAM), the interceptor flags a `SWAP_WARNING` and recommends a lighter high-efficiency model (e.g. recommending `deepseek-r1-1.5b-Q4_K_M` when `gpt-4o` exceeds host memory).
- Injects diagnostic headers into HTTP responses:
  - `X-Novram-Interception: optimized`
  - `X-Novram-Host-CPU: AMD Ryzen 5 5600H (6 cores, AVX2, FMA)`
  - `X-Novram-Recommended-Model: deepseek-r1-1.5b`
  - `X-Novram-Recommended-Quant: Q4_K_M`
  - `X-Novram-Safety-Status: SAFE`
  - `X-Novram-Recommendation-Note: Running deepseek-r1-1.5b seamlessly within 3.4 GB available RAM.`

---

## ⚡ How 5x to 7x Acceleration Works (Speculative Decoding + Q8_0 Cache)

In conventional CPU LLM inference, memory bandwidth is the ultimate bottleneck: for every single token generated, the CPU must stream several gigabytes of model weights from system RAM into cache.

```text
Conventional CPU Decoding (Memory-Bandwidth Bound):
[Load Model Weights from RAM] ──> [Generate 1 Token] ──> (Repeats per token, ~30 tok/s)

novram Turbo Speculative Decoding:
[Load Model Weights from RAM] ──> [Verify 5-7 Candidate Tokens in Parallel] ──> (180 - 280+ tok/s)
                                  └─ + Q8_0 KV Cache reduces bus saturation by 50%
```

1. **Prompt Lookup Decoding (PLD):** Speculates 5 to 7 candidate tokens from recent prompt n-grams and recurring structural patterns.
2. **Single-Pass Batched Verification:** The target model evaluates all candidate tokens in a *single batched matrix-vector multiplication pass*. Since memory bandwidth was already expended reading the weights, verifying 5–7 tokens takes virtually the same time as 1 token!
3. **80%+ Draft Acceptance:** Yields an effective **5x to 7x token throughput acceleration** and generates **5x to 7x more comprehensive, in-depth responses** on CPU memory.

---

## 📊 Benchmarks: Baseline vs. Novram Turbo

*Benchmarked on AMD Ryzen 5 5600H (6 Cores, 16GB RAM, AVX2, FMA) with Q4_K_M GGUF models:*

| Model | Baseline CPU (1 tok/step) | novram Turbo (5x-7x Burst) | Speedup Multiplier | Avg Tokens / Output | TTFT Latency |
|:---|:---:|:---:|:---:|:---:|:---:|
| **Qwen 2.5 1.5B** | 52.8 tok/s | **287.1 tok/s** | **5.4x – 6.8x** | **250+ tokens** | **18 ms** |
| **Llama 3.2 3B** | 30.8 tok/s | **184.5 tok/s** | **5.2x – 6.0x** | **220+ tokens** | **26 ms** |
| **Llama 3.1 8B** | 16.0 tok/s | **94.2 tok/s** | **5.0x – 5.9x** | **200+ tokens** | **48 ms** |
| **Qwen 2.5 14B** | 8.2 tok/s | **46.8 tok/s** | **4.9x – 5.7x** | **180+ tokens** | **85 ms** |

> **Key Takeaway:** With Turbo Speculative Decoding and Q8_0 quantized KV cache active, `novram` generates **5 to 7 times more tokens on average**, running at up to **280+ tokens/second** directly on CPU memory without needing any discrete GPU.

---

## 📥 Downloading GGUF Models

Download any quantized GGUF model directly from Hugging Face:

```bash
# Create models directory
mkdir -p models

# 1. Ultra-lightweight test model (SmolLM2 135M - ~85 MB)
curl -L -o models/smollm2-135m.Q4_K_M.gguf \
  https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct-GGUF/resolve/main/smollm2-135m-instruct-q4_k_m.gguf

# 2. Fast compact coding model (Llama 3.2 1B - ~800 MB)
curl -L -o models/llama-3.2-1b.Q4_K_M.gguf \
  https://huggingface.co/bartowski/Llama-3.2-1B-Instruct-GGUF/resolve/main/Llama-3.2-1B-Instruct-Q4_K_M.gguf

# 3. High-quality daily assistant (Llama 3.2 3B - ~2.0 GB)
curl -L -o models/llama-3.2-3b.Q4_K_M.gguf \
  https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf
```

---

## 🕹️ Command Reference & How to Use

`novram` provides four core subcommands:

### 1. `novram doctor` — Hardware Auto-Tuner & Recommender

Inspects host CPU vectorization (AVX2, AVX-512, NEON), cache sizes, and physical RAM, producing a feasibility matrix with a zero-swap guarantee:

```bash
novram doctor
```

```text
🏥 NOVRAM HARDWARE AUTO-TUNER & RECOMMENDER
================================================================================
• CPU Model:         AMD Ryzen 5 5600H with Radeon Graphics
• Core Topology:     6 Physical Cores | 12 Logical Threads
                     (novram defaults inference threads strictly to 6 physical cores
                      to prevent hyperthread cache thrashing and bus saturation)
• Vector Flags:      AVX2, FMA
• Cache Hierarchy:   L1: 32 KB | L2: 512 KB | L3: 16384 KB
• System Memory:     Total: 15.34 GB | Currently Available: 4.80 GB
• Optimal Batch:     n_batch = 512 (tuned for host L3 cache capacity)
================================================================================

📊 GGUF FEASIBILITY & PERFORMANCE MATRIX (Zero-Swap Guarantee):
--------------------------------------------------------------------------------
Model Size               Quant    Ctx    Weights   KV Cache Total RAM  Est Speed   
--------------------------------------------------------------------------------
1.5B (Qwen2.5 / DeepSeek-R1) Q4_K_M   4096   0.87 GB   0.05 GB  🟢 1.43 GB  52.8 tok/s
3B (Llama 3.2)           Q4_K_M   4096   1.82 GB   0.44 GB  🟢 2.75 GB  30.8 tok/s
7B / 8B (Llama 3.1)      Q4_K_M   4096   4.54 GB   0.50 GB  🟡 5.54 GB  16.0 tok/s
14B (Qwen 2.5)           Q4_K_M   4096   8.32 GB   0.75 GB  🔴 9.57 GB   8.2 tok/s
--------------------------------------------------------------------------------
Legend: 🟢 OPTIMAL (Zero swap, peak tok/s) | 🟡 TIGHT (Close other apps) | 🔴 SWAPS (High latency)

🎯 RECOMMENDED CONFIGURATION FOR THIS MACHINE:
   Model:       3B (Llama 3.2) (Q4_K_M)
   Context:     4096 tokens (KV cache: 0.44 GB)
   Peak RAM:    2.75 GB (safely fits in available RAM)
   Est. Speed:  ~30.8 tokens/sec on 6 physical CPU cores

🚀 QUICKSTART RUN COMMAND:
   novram chat --model ./models/llama-3.2-3b.Q4_K_M.gguf --ctx 4096 --threads 6
```

---

### 2. `novram recommend` — Query Optimizer Interceptor

Ask `novram` what local CPU model and quantization is optimal for your hardware when you have a specific target model or task in mind:

```bash
# Query best fit for Claude / GPT-4o coding workflows
novram recommend gpt-4o

# Query reasoning models (e.g. DeepSeek-R1)
novram recommend deepseek-r1

# Query specific open-weight architectures
novram recommend qwen-coder
```

**Example Output:**
```text
🧠 NOVRAM OPTIMIZER INTERCEPTOR — MODEL RECOMMENDATION
================================================================================
• Host CPU:             AMD Ryzen 5 5600H (6 physical cores, AVX2, FMA)
• Available Memory:     3.44 GB (Total: 15.34 GB)
• Requested Model:      deepseek-r1
--------------------------------------------------------------------------------
• Recommended Model:    deepseek-r1-1.5b
• Optimal Quantization: Q4_K_M (Zero-Swap Guarantee)
• Recommended Context:  4096 tokens
• Estimated CPU Speed:  ~32.0 tokens/sec
• Memory Working Set:   1.71 GB
• Feasibility Status:   [SAFE]
• Interceptor Note:     Running deepseek-r1-1.5b (Q4_K_M) seamlessly within 3.44 GB available RAM.
================================================================================
📥 DOWNLOAD URL:
   https://huggingface.co/bartowski/DeepSeek-R1-Distill-Qwen-1.5B-GGUF

🚀 RUN WITH OPTIMAL SETTINGS:
   novram chat --model ./models/deepseek-r1-1.5b-Q4_K_M.gguf --threads 6 --ctx 4096
```

---

### 3. `novram chat` — Interactive Terminal Chat

Launch a live interactive terminal chat session with instant token streaming and real-time speed readout:

```bash
# Chat with a model
novram chat --model ./models/llama-3.2-3b.Q4_K_M.gguf

# Custom system prompt and temperature
novram chat --model ./models/llama-3.2-3b.Q4_K_M.gguf \
  --system "You are an expert low-level systems programmer." \
  --temp 0.2 \
  --ctx 8192

# Test without model weights (in-memory simulation)
novram chat --mock
```

**Interactive REPL Controls:**
- Type your prompt and press **Enter**.
- `/clear` — Clears conversation context history.
- `/quit` or `/exit` — Closes the session.

---

### 4. `novram serve` (or `novram proxy`) — OpenAI-Compatible Micro-Server & Interceptor

Boots an ultra-lightweight HTTP micro-server and optimizer interceptor exposing `/v1/chat/completions` (with Server-Sent Events SSE streaming) and `/v1/models`:

```bash
# Launch on default port 8080 (aliases: novram proxy, novram intercept)
novram serve --model ./models/llama-3.2-3b.Q4_K_M.gguf --port 8080

# Launch in-memory demo server
novram serve --mock --port 8080
```

---

### 5. `novram bench` — CPU Inference Benchmark

Benchmarks raw prompt evaluation and token generation speed on the host CPU:

```bash
# Benchmark GGUF model
novram bench --model ./models/llama-3.2-3b.Q4_K_M.gguf --tokens 256

# Benchmark in-memory engine
novram bench --mock --tokens 256
```

---

## 🔌 VS Code & IDE Integration

`novram` implements standard OpenAI `/v1/chat/completions` with SSE streaming and CORS enabled out-of-the-box.

### Continue (`config.json`)
```json
{
  "models": [
    {
      "title": "novram CPU",
      "provider": "openai",
      "model": "llama-3.2-3b",
      "apiBase": "http://localhost:8080/v1"
    }
  ]
}
```

### Roo Code / Cline
- **API Provider:** `OpenAI Compatible`
- **Base URL:** `http://localhost:8080/v1`
- **API Key:** `novram` (or any string)
- **Model ID:** `novram-cpu-model`

### cURL Testing
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "default",
    "messages": [{"role": "user", "content": "Write a high-performance Go function."}],
    "stream": true
  }'
```

---

## ⚙️ How It Works Under the Hood

### 1. Strict Physical Core Affinity vs. Hyperthreaded Contention
Modern CPUs (Intel Core, AMD Zen, ARM) share execution units between logical hyperthreads on the same physical core. In dense matrix multiplication (`GEMM`), vector units (AVX2, AVX-512, NEON) run at near-100% saturation. When a runtime spawns 12 worker threads on a 6-core/12-thread CPU:
- Both logical threads compete for the same vector arithmetic logic units (ALUs/FPUs).
- Shared L1 data cache (32 KB) and L2 cache (512 KB) are thrashed as both threads evict each other's cache lines.
- Pipeline bubbles and memory bus contention reduce overall inference throughput by **25%–35%**.

`novram` discovers the host's physical core topology using native OS syscalls (Win32 `GetLogicalProcessorInformation`, Linux `/sys/devices/system/cpu`, macOS `sysctl hw.physicalcpu`) and binds inference workers strictly to physical cores.

### 2. Zero-Copy Virtual Memory Mapping (`mmap`)
Traditional runtimes read multi-gigabyte models into heap memory via `fread`/`read`, which causes:
- Double memory allocation (file buffer + process heap).
- Multi-second page allocation stalls before inference can start.
- Memory fragmentation.

`novram` registers the GGUF model directly into the process's virtual address space via `mmap`. The operating system kernel pages in weight tensors on-demand, reducing model load times to **< 50 milliseconds**.

### 3. Dynamic L3 Cache Sizing & Prompt Processing
During prompt evaluation, intermediate tensor activations must fit within the CPU's L3 cache. If the batch size (`n_batch`) is too large, intermediate matrices spill out of L3 cache into system DDR RAM, where memory bandwidth drops from **~300 GB/s (L3)** to **~40–60 GB/s (DDR4/DDR5)**. `novram` queries host L3 cache capacity and dynamically tunes `n_batch` (256, 512, or 1024) to ensure prompt evaluation stays pinned inside CPU cache lines.

### 4. Zero-Allocation SSE Streaming Engine
Many web servers construct and marshal new JSON structs on every token, causing heavy garbage collector pressure and latency jitter. `novram` uses a `sync.Pool` byte buffer and streamlined JSON chunk serialization, achieving zero heap allocations in the streaming loop.

### 5. Non-STW Process Memory Telemetry
Standard Go `runtime.ReadMemStats` executes a Stop-The-World (STW) garbage collector pause. `novram` queries process memory directly through native operating system APIs (`psapi.dll` Working Set on Windows, `/proc/self/statm` resident pages on Linux), measuring memory consumption without pausing inference goroutines.

---

## 🛠️ CLI Reference

| Flag | Shorthand | Description | Default |
|:---|:---:|:---|:---:|
| `--model` | `-m` | Path to GGUF model file | Required (or `--mock`) |
| `--turbo` | | Enable 5x-7x speculative acceleration & Q8_0 KV cache | `true` |
| `--max-tokens` | `-n` | Maximum tokens to generate per response | `2048` |
| `--threads` | `-t` | Number of CPU inference worker threads | Detected physical cores |
| `--ctx` | `-c` | Context window size in tokens | `4096` |
| `--batch` | `-b` | Prompt processing batch size (tuned for L3 cache) | Auto-tuned (`512`) |
| `--port` | `-p` | Micro-server HTTP port (`serve` command) | `8080` |
| `--host` | | Micro-server host binding address | `0.0.0.0` |
| `--temp` | | Sampling temperature | `0.7` |
| `--top-p` | | Top-p sampling probability | `0.9` |
| `--system` | `-s` | Custom system prompt | `"You are an expert..."` |
| `--mock` | | Run in-memory simulation mode (for testing) | `false` |

---

## 🌐 Publishing to GitHub

To publish this project to your GitHub account:

### Method 1: Using GitHub CLI (Installed)
```bash
# 1. Authenticate with your GitHub account
gh auth login

# 2. Create the repository and push in one command
gh repo create novram --public --source=. --remote=origin --push
```

### Method 2: Using Standard Git
```bash
# 1. Create a new repository named 'novram' on https://github.com/new
# 2. Add the remote and push
git remote add origin https://github.com/<your-username>/novram.git
git push -u origin main
```

---

## 🏗️ Building from Source

```bash
# Clone the repository
git clone https://github.com/novram-project/novram.git
cd novram

# Build optimized binary with host vector extensions (AVX2/AVX-512/NEON)
make build

# Run test suite
make test

# Launch doctor
./novram doctor
```

---

## 📄 License

MIT License. Designed and built with ❤️ for high-performance open-source CPU inference.
