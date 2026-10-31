# novram - Ultra-fast CPU-only LLM runner & micro-server
# Cross-platform build automation for Windows, Linux, and macOS

BINARY_NAME := novram
CMD_DIR := ./cmd/novram
BUILD_DIR := ./bin

# Go toolchain
GO ?= go
CGO_ENABLED ?= 1
GO_LDFLAGS := -ldflags="-s -w -X main.Version=0.1.0"

# Detect Host Operating System & Architecture
ifeq ($(OS),Windows_NT)
    HOST_OS := windows
    BINARY_EXT := .exe
    RM := cmd /C del /Q /F 2>nul || exit 0
    RMDIR := cmd /C rmdir /S /Q 2>nul || exit 0
    INSTALL_DIR := $(LOCALAPPDATA)\Programs\novram
else
    UNAME_S := $(shell uname -s 2>/dev/null || echo Unknown)
    ifeq ($(UNAME_S),Darwin)
        HOST_OS := darwin
    else
        HOST_OS := linux
    endif
    BINARY_EXT :=
    RM := rm -f
    RMDIR := rm -rf
    INSTALL_DIR := /usr/local/bin
endif

GO_ARCH := $(shell $(GO) env GOARCH 2>/dev/null || echo amd64)

# Platform-aware vector optimization flags
ifeq ($(GO_ARCH),arm64)
    # ARM NEON vectorization
    CGO_CFLAGS := -O3
    CGO_CXXFLAGS := -O3 -std=c++17
else
    # x86_64 AVX2 / FMA vectorization
    CGO_CFLAGS := -O3 -march=native -mavx2 -mfma
    CGO_CXXFLAGS := -O3 -march=native -mavx2 -mfma -std=c++17
endif

TARGET_BINARY := $(BINARY_NAME)$(BINARY_EXT)

.PHONY: all build build-static test doctor bench clean install

all: build

build:
	@echo "==> Building $(TARGET_BINARY) for $(HOST_OS)/$(GO_ARCH)..."
	CGO_ENABLED=$(CGO_ENABLED) CGO_CFLAGS="$(CGO_CFLAGS)" CGO_CXXFLAGS="$(CGO_CXXFLAGS)" $(GO) build $(GO_LDFLAGS) -o $(TARGET_BINARY) $(CMD_DIR)
	@echo "==> Build complete: $(TARGET_BINARY)"

build-static:
	@echo "==> Building standalone zero-dependency static binary..."
	CGO_ENABLED=0 $(GO) build $(GO_LDFLAGS) -o $(TARGET_BINARY) $(CMD_DIR)
	@echo "==> Standalone static build complete: $(TARGET_BINARY)"

test:
	@echo "==> Running cross-platform test suite..."
	$(GO) test -v ./...

doctor: build
	@echo "==> Running hardware auto-tuner..."
	./$(TARGET_BINARY) doctor

bench: build
	@echo "==> Running CPU inference benchmark..."
	./$(TARGET_BINARY) bench --mock

clean:
	@echo "==> Cleaning build artifacts..."
	@$(RM) $(TARGET_BINARY) novram novram.exe
	@$(RMDIR) $(BUILD_DIR)

install: build
ifeq ($(HOST_OS),windows)
	@echo "==> Installing $(TARGET_BINARY) to $(INSTALL_DIR)..."
	@powershell -Command "New-Item -ItemType Directory -Force -Path '$(INSTALL_DIR)' | Out-Null; Copy-Item '$(TARGET_BINARY)' '$(INSTALL_DIR)\$(TARGET_BINARY)' -Force"
else
	@echo "==> Installing $(TARGET_BINARY) to $(INSTALL_DIR)..."
	@install -m 755 $(TARGET_BINARY) $(INSTALL_DIR)/$(BINARY_NAME) 2>/dev/null || cp $(TARGET_BINARY) $(INSTALL_DIR)/$(BINARY_NAME)
endif
