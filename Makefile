# Go Database Engine Makefile

# Variables
BINARY_NAME=go-database
MAIN_PATH=./cmd/go-database
PKG_LIST=$(shell go list ./...)
GO_FILES=$(shell find . -name '*.go' | grep -v vendor | grep -v .git)

# Build targets
.PHONY: all build clean test coverage lint fmt vet

# Default target
all: fmt vet test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@go clean
	@rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint code
lint:
	@echo "Running linters..."
	@golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	@go vet ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Development setup
dev-setup:
	@echo "Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  test-race   - Run tests with race detector"
	@echo "  coverage    - Run tests with coverage report"
	@echo "  lint        - Run linters"
	@echo "  fmt         - Format code"
	@echo "  vet         - Vet code"
	@echo "  bench       - Run benchmarks"
	@echo "  deps        - Install dependencies"
	@echo "  dev-setup   - Setup development environment"
	@echo "  help        - Show this help"