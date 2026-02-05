.PHONY: build install test lint clean check all

# Build the ang CLI
build:
	@echo "--- Building ANG Compiler ---"
	go build -o bin/ang ./cmd/ang

# Install ang globally
install:
	@echo "--- Installing ANG CLI ---"
	go install ./cmd/ang

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
