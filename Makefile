# novram - Ultra-fast CPU-only LLM runner & micro-server
# Build automation with high-performance CPU vector optimizations

BINARY_NAME := novram
CMD_DIR := ./cmd/novram
BUILD_DIR := ./bin

# Compiler flags
GO := go
CGO_ENABLED ?= 1
GO_LDFLAGS := -ldflags="-s -w -X main.Version=0.1.0"
CGO_CFLAGS := -O3 -march=native -mavx2 -mfma
CGO_CXXFLAGS := -O3 -march=native -mavx2 -mfma -std=c++17

.PHONY: all build build-static test doctor bench clean install

all: build

build:
	@echo "==> Building $(BINARY_NAME) with CPU vector flags..."
	CGO_ENABLED=$(CGO_ENABLED) CGO_CFLAGS="$(CGO_CFLAGS)" CGO_CXXFLAGS="$(CGO_CXXFLAGS)" $(GO) build $(GO_LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)
	@echo "==> Build complete: $(BINARY_NAME)"

build-static:
	@echo "==> Building standalone static binary..."
	CGO_ENABLED=0 $(GO) build $(GO_LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)
	@echo "==> Standalone static build complete: $(BINARY_NAME)"

test:
	@echo "==> Running unit and integration tests..."
	$(GO) test -v ./...

doctor: build
	@echo "==> Running hardware auto-tuner..."
	./$(BINARY_NAME) doctor

bench: build
	@echo "==> Running CPU inference benchmark..."
	./$(BINARY_NAME) bench --mock

clean:
	@echo "==> Cleaning build artifacts..."
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe
	@rm -rf $(BUILD_DIR)

install: build
	@echo "==> Installing $(BINARY_NAME) to /usr/local/bin..."
	@install -m 755 $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME) 2>/dev/null || cp $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
