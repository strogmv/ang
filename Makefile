.PHONY: build install test lint clean check all mcp-gen release-smoke benchmark-3iter determinism-plan-apply determinism-matrix

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

# Generate MCP server from template
mcp-gen:
	@echo "--- Generating MCP server ---"
	./scripts/gen_mcp_server.sh

# Run release smoke checks (Go+Python outputs)
release-smoke:
	@echo "--- Release Smoke ---"
	./scripts/release-smoke.sh

# Run reproducible 3-iteration marketplace benchmark
benchmark-3iter: build
	@echo "--- 3-Iteration Backend Benchmark ---"
	./benchmarks/three_iteration_marketplace/run.sh

# Verify deterministic output across plan/apply phases (release mode)
determinism-plan-apply:
	@echo "--- Determinism Plan/Apply ---"
	bash ./scripts/ci_determinism_plan_apply.sh

# Cross-platform-friendly determinism check for phases plan/apply/all
determinism-matrix:
	@echo "--- Determinism Matrix (plan/apply/all) ---"
	go run ./scripts/determinism_ci
