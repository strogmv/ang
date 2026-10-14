.PHONY: build install test lint clean check all release-smoke benchmark-3iter determinism-matrix

# Build the ang CLI
build:
	@echo "--- Building ANG Compiler ---"
	go build -o bin/ang ./cmd/ang

# Install ang to $(go env GOBIN) or $(go env GOPATH)/bin — add that dir to PATH
install:
	@echo "--- Installing ANG CLI ---"
	go install ./cmd/ang
	@BIN=$$(go env GOBIN); \
	if [ -z "$$BIN" ]; then BIN=$$(go env GOPATH)/bin; fi; \
	echo "Installed: $$BIN/ang"; \
	echo "On PATH if you have: export PATH=\"$$PATH:$$BIN\""

# Run compiler tests
test:
	@echo "--- Testing Compiler ---"
	go test -v ./compiler/...

# Lint Go code
lint:
	@echo "--- Linting ---"
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/

# Run all checks
check: lint test

# Build and test
all: build test
	@echo "--- Build SUCCESSFUL ---"

# Run release smoke checks (Go output)
release-smoke:
	@echo "--- Release Smoke ---"
	./scripts/release-smoke.sh

# Run reproducible 3-iteration marketplace benchmark
benchmark-3iter: build
	@echo "--- 3-Iteration Backend Benchmark ---"
	./benchmarks/three_iteration_marketplace/run.sh

# Determinism check for phases plan/apply/all on a scratch copy of examples/blog
determinism-matrix:
	@echo "--- Determinism Matrix (plan/apply/all) ---"
	go run ./scripts/determinism_ci
