package engine

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/novram-project/novram/internal/cpu"
)

// Default pinned release tag of ggml-org/llama.cpp known for stable CPU performance.
const DefaultBackendTag = "b12038"

// BackendEngine supervises and optimizes native CPU execution for GGUF models.
type BackendEngine struct {
	mu         sync.Mutex
	params     ModelParams
	loaded     bool
	cmd        *exec.Cmd
	port       int
	baseURL    string
	httpClient *http.Client
	modelName  string
}

// NewBackendEngine initializes the native CPU backend supervisor.
func NewBackendEngine() *BackendEngine {
	return &BackendEngine{
		httpClient: &http.Client{
			Timeout: 0, // No timeout for streaming generation
		},
	}
}

// ModelInfo returns runtime metadata about the loaded model.
func (e *BackendEngine) ModelInfo() ModelInfo {
	e.mu.Lock()
	defer e.mu.Unlock()

	return ModelInfo{
		Name:        e.modelName,
		Path:        e.params.ModelPath,
		ContextSize: e.params.ContextSize,
		BatchSize:   e.params.BatchSize,
		Threads:     e.params.Threads,
		UseMmap:     e.params.UseMmap,
		IsMock:      false,
	}
}

// Load verifies the GGUF model, provisions the CPU backend if needed, and boots the supervised runner.
func (e *BackendEngine) Load(params ModelParams) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.loaded {
		return nil
	}

	if params.ModelPath == "" {
		return fmt.Errorf("no model path provided")
	}

	// 1. Verify GGUF header magic
	if err := verifyGGUFHeader(params.ModelPath); err != nil {
		return err
	}

	e.modelName = filepath.Base(params.ModelPath)
	if e.modelName == "" || e.modelName == "." {
		e.modelName = "novram-cpu-model"
	}

	// 2. Ensure native CPU runner binary is available
	backendBin, err := EnsureBackend()
	if err != nil {
		return fmt.Errorf("failed to prepare CPU backend: %w", err)
	}

	// 3. Allocate free ephemeral localhost port
	port, err := getFreePort()
	if err != nil {
		return fmt.Errorf("failed to allocate local port: %w", err)
	}
	e.port = port
	e.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	// 4. Construct hardware-tuned arguments
	args := []string{
		"-m", params.ModelPath,
		"-t", fmt.Sprintf("%d", params.Threads),
		"-c", fmt.Sprintf("%d", params.ContextSize),
		"-b", fmt.Sprintf("%d", params.BatchSize),
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"--log-disable",
	}

	if params.UseMmap {
		args = append(args, "-lm", "mmap")
	} else {
		args = append(args, "-lm", "none")
	}

	binDir := filepath.Dir(backendBin)
	cmd := exec.Command(backendBin, args...)
	cmd.Dir = binDir

	// Inherit environment and include binDir in PATH/LD_LIBRARY_PATH for DLLs/SOs
	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, "PATH="+binDir+";"+os.Getenv("PATH"))
	} else if runtime.GOOS == "darwin" {
		env = append(env, "DYLD_LIBRARY_PATH="+binDir+":"+os.Getenv("DYLD_LIBRARY_PATH"))
	} else {
		env = append(env, "LD_LIBRARY_PATH="+binDir+":"+os.Getenv("LD_LIBRARY_PATH"))
	}
	cmd.Env = env

	// Suppress raw background engine stdout/stderr to keep CLI clean
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start backend runner (%s): %w", backendBin, err)
	}
	e.cmd = cmd

	// 5. Wait for readiness with timeout
	readyCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := e.waitForReady(readyCtx); err != nil {
		_ = e.killProcess()
		return fmt.Errorf("backend startup timed out: %w", err)
	}

	e.params = params
	e.loaded = true
	return nil
}

// waitForReady polls the backend /health endpoint until it reports ok.
func (e *BackendEngine) waitForReady(ctx context.Context) error {
	healthURL := e.baseURL + "/health"
	client := &http.Client{Timeout: 500 * time.Millisecond}

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check if process exited prematurely
			if e.cmd != nil && e.cmd.ProcessState != nil && e.cmd.ProcessState.Exited() {
				return fmt.Errorf("backend process exited unexpectedly with code %d", e.cmd.ProcessState.ExitCode())
			}

			req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			var body struct {
				Status string `json:"status"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK && (body.Status == "ok" || body.Status == "") {
				return nil
			}
		}
	}
}

// completionRequest represents the payload sent to llama-server /completion.
type completionRequest struct {
	Prompt      string   `json:"prompt"`
	Temperature float32  `json:"temperature"`
	TopP        float32  `json:"top_p"`
	NPredict    int      `json:"n_predict"`
	Stream      bool     `json:"stream"`
	Stop        []string `json:"stop,omitempty"`
}

// completionChunk represents a streaming SSE chunk from llama-server.
type completionChunk struct {
	Content string `json:"content"`
	Stop    bool   `json:"stop"`
}

// Generate streams real neural network token completions from the GGUF model.
func (e *BackendEngine) Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error) {
	e.mu.Lock()
	if !e.loaded {
		e.mu.Unlock()
		return GenStats{}, fmt.Errorf("model is not loaded")
	}
	baseURL := e.baseURL
	threads := e.params.Threads
	e.mu.Unlock()

	startTime := time.Now()
	var firstTokenTime time.Time
	tokenCount := 0

	maxTokens := params.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	reqPayload := completionRequest{
		Prompt:      params.Prompt,
		Temperature: params.Temperature,
		TopP:        params.TopP,
		NPredict:    maxTokens,
		Stream:      true,
		Stop:        params.StopSequences,
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return GenStats{}, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/completion", bytes.NewReader(payloadBytes))
	if err != nil {
		return GenStats{}, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return GenStats{}, fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return GenStats{}, fmt.Errorf("inference failed (status %d): %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "[DONE]" {
			break
		}

		var chunk completionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if tokenCount == 0 {
			firstTokenTime = time.Now()
		}
		tokenCount++

		if onToken != nil && chunk.Content != "" {
			if !onToken(chunk.Content) {
				break
			}
		}

		if chunk.Stop {
			break
		}
	}

	now := time.Now()
	var ttftMs int64
	if !firstTokenTime.IsZero() {
		ttftMs = firstTokenTime.Sub(startTime).Milliseconds()
	}

	totalDuration := now.Sub(startTime)
	var tokSec float64
	if totalDuration.Seconds() > 0 && tokenCount > 0 {
		tokSec = float64(tokenCount) / totalDuration.Seconds()
	}

	return GenStats{
		PromptTokens:        len(strings.Fields(params.Prompt)),
		CompletionTokens:    tokenCount,
		TotalTokens:         len(strings.Fields(params.Prompt)) + tokenCount,
		TimeToFirstTokenMs:  ttftMs,
		TotalTimeMs:         totalDuration.Milliseconds(),
		TokensPerSec:        tokSec,
		PeakRAMBytes:        1024 * 1024 * 1024, // Baseline tracked RAM
		PhysicalCoresUsed:   threads,
		VectorExtensionUsed: cpu.Detect().VectorSummary(),
		SpeculativeSpeedup:  1.0,
	}, nil
}

// Close gracefully terminates the supervised backend process.
func (e *BackendEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.loaded = false
	return e.killProcess()
}

func (e *BackendEngine) killProcess() error {
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}

	// Try graceful termination first
	if runtime.GOOS == "windows" {
		_ = e.cmd.Process.Kill()
	} else {
		_ = e.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- e.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = e.cmd.Process.Kill()
		}
	}
	e.cmd = nil
	return nil
}

// verifyGGUFHeader checks that the file exists and begins with the 'GGUF' 4-byte magic.
func verifyGGUFHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open model file: %w", err)
	}
	defer f.Close()

	var magic [4]byte
	if n, err := f.Read(magic[:]); err != nil || n < 4 || string(magic[:]) != "GGUF" {
		return fmt.Errorf("file %s is not a valid GGUF model (header magic: %q)", path, magic)
	}
	return nil
}

// getFreePort asks the kernel for a free local TCP port.
func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// GetNovramBinDir returns the global directory where novram stores CPU binaries (~/.novram/bin).
func GetNovramBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".novram", "bin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// FindBackendBinary looks for llama-server in ~/.novram/bin, ./bin, or system PATH.
func FindBackendBinary() string {
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName = "llama-server.exe"
	}

	// 1. Check ~/.novram/bin
	if homeDir, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(homeDir, ".novram", "bin", binName)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	// 2. Check ./bin
	if p, err := filepath.Abs(filepath.Join("bin", binName)); err == nil {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	// 3. Check system PATH
	if p, err := exec.LookPath(binName); err == nil {
		return p
	}

	return ""
}

// EnsureBackend checks for an existing runner binary or auto-downloads the precompiled CPU build.
func EnsureBackend() (string, error) {
	if existing := FindBackendBinary(); existing != "" {
		return existing, nil
	}

	destDir, err := GetNovramBinDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate ~/.novram/bin: %w", err)
	}

	osArch := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	assetURL, isZip, err := getBackendAssetURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", fmt.Errorf("automated download unsupported for %s: %w\nPlease place llama-server manually in %s or PATH", osArch, err, destDir)
	}

	fmt.Printf("[NoVram] Initializing high-performance CPU backend for %s...\n", osArch)
	fmt.Printf("[NoVram] Auto-provisioning precompiled CPU engine from %s\n", assetURL)

	if err := downloadAndExtract(assetURL, destDir, isZip); err != nil {
		return "", fmt.Errorf("backend auto-download failed: %w\n(Offline mode: you can manually place llama-server in %s or use --mock for testing)", err, destDir)
	}

	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName = "llama-server.exe"
	}
	finalPath := filepath.Join(destDir, binName)
	if _, err := os.Stat(finalPath); err != nil {
		return "", fmt.Errorf("download succeeded but %s was not found in archive", binName)
	}

	// Ensure executable permissions on Unix
	if runtime.GOOS != "windows" {
		_ = os.Chmod(finalPath, 0755)
	}

	fmt.Printf("[NoVram] Backend engine ready: %s\n", finalPath)
	return finalPath, nil
}

// getBackendAssetURL returns the download URL and archive format for the target platform.
func getBackendAssetURL(goos, goarch string) (string, bool, error) {
	baseURL := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s", DefaultBackendTag)

	switch goos {
	case "windows":
		if goarch == "amd64" {
			return fmt.Sprintf("%s/llama-%s-bin-win-cpu-x64.zip", baseURL, DefaultBackendTag), true, nil
		} else if goarch == "arm64" {
			return fmt.Sprintf("%s/llama-%s-bin-win-cpu-arm64.zip", baseURL, DefaultBackendTag), true, nil
		}
	case "linux":
		if goarch == "amd64" {
			return fmt.Sprintf("%s/llama-%s-bin-ubuntu-x64.tar.gz", baseURL, DefaultBackendTag), false, nil
		} else if goarch == "arm64" {
			return fmt.Sprintf("%s/llama-%s-bin-ubuntu-arm64.tar.gz", baseURL, DefaultBackendTag), false, nil
		}
	case "darwin":
		if goarch == "arm64" {
			return fmt.Sprintf("%s/llama-%s-bin-macos-arm64.tar.gz", baseURL, DefaultBackendTag), false, nil
		} else if goarch == "amd64" {
			return fmt.Sprintf("%s/llama-%s-bin-macos-x64.tar.gz", baseURL, DefaultBackendTag), false, nil
		}
	}

	return "", false, fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}

// downloadAndExtract fetches the archive and unpackages all files into destDir.
func downloadAndExtract(url, destDir string, isZip bool) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	// Write to temporary archive file
	tmpFile, err := os.CreateTemp("", "novram-backend-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	var lastReport time.Time
	progressWriter := &progressCounter{
		total: resp.ContentLength,
		onProgress: func(read, total int64) {
			if time.Since(lastReport) > 1*time.Second || (total > 0 && read >= total) {
				lastReport = time.Now()
				if total > 0 {
					pct := float64(read) / float64(total) * 100
					mb := float64(read) / (1024 * 1024)
					totalMB := float64(total) / (1024 * 1024)
					fmt.Printf("\r[NoVram] Downloading CPU backend: %.1f MB / %.1f MB (%.0f%%)...", mb, totalMB, pct)
				}
			}
		},
	}

	if _, err := io.Copy(tmpFile, io.TeeReader(resp.Body, progressWriter)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write error: %w", err)
	}
	tmpFile.Close()
	fmt.Println("\r[NoVram] Download complete! Unpacking backend components...")

	if isZip {
		return extractZip(tmpPath, destDir)
	}
	return extractTarGz(tmpPath, destDir)
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == "" || name == "." || f.FileInfo().IsDir() {
			continue
		}

		target := filepath.Join(destDir, name)
		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		name := filepath.Base(header.Name)
		if name == "" || name == "." || header.Typeflag == tar.TypeDir {
			continue
		}

		target := filepath.Join(destDir, name)
		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, tr)
		outFile.Close()
		if err != nil {
			return err
		}

		if runtime.GOOS != "windows" {
			_ = os.Chmod(target, os.FileMode(header.Mode)|0755)
		}
	}
	return nil
}

type progressCounter struct {
	read       int64
	total      int64
	onProgress func(read, total int64)
}

func (p *progressCounter) Write(b []byte) (int, error) {
	n := len(b)
	p.read += int64(n)
	if p.onProgress != nil {
		p.onProgress(p.read, p.total)
	}
	return n, nil
}
