# Go Microservices with gRPC and Messaging
# ==========================================
# This Makefile provides commands for building, running, and testing the project.

.PHONY: all build run-payment run-order run-all test clean proto help

# Default target
all: build

# ===== BUILD =====

# Build all services
build:
	@echo "Building all services..."
	@go build -o bin/payment ./services/payment/cmd
	@go build -o bin/order ./services/order/cmd
	@echo "Build complete! Binaries in ./bin/"

# Build individual services
build-payment:
	@go build -o bin/payment ./services/payment/cmd

build-order:
	@go build -o bin/order ./services/order/cmd

# ===== RUN =====

# Run Payment service (gRPC on :50051)
run-payment:
	@echo "Starting Payment Service (gRPC :50051)..."
	@go run ./services/payment/cmd

# Run Order service (HTTP on :8080)
run-order:
	@echo "Starting Order Service (HTTP :8080)..."
	@go run ./services/order/cmd

# Run all services (requires multiple terminals or background processes)
run-all:
	@echo "Starting all services..."
	@echo "Run these commands in separate terminals:"
	@echo "  make run-payment"
	@echo "  make run-order"

# ===== TEST =====

# Run all tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with race detector
test-race:
	@go test -race ./...

# Run tests with coverage
test-coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ===== DEVELOPMENT =====

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@golangci-lint run

# Clean build artifacts
clean:
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Cleaned!"

# ===== PROTO =====

# Generate protobuf code (requires protoc)
proto:
	@echo "Generating protobuf code..."
	@protoc --go_out=. --go-grpc_out=. proto/payment/payment.proto
	@protoc --go_out=. --go-grpc_out=. proto/order/order.proto
	@echo "Done!"

# ===== DEMO =====

# Demo: Create an order (requires services running)
demo:
	@echo "Creating a test order..."
	@curl -X POST http://localhost:8080/orders \
		-H "Content-Type: application/json" \
		-d '{"customer_email":"demo@example.com","items":[{"product_name":"Laptop Pro","quantity":1,"unit_price_cents":249900}]}' | jq .

# Demo: List orders
demo-list:
	@curl -s http://localhost:8080/orders | jq .

# Demo: Check health
demo-health:
	@curl -s http://localhost:8080/health | jq .

# Demo: Get stats
demo-stats:
	@curl -s http://localhost:8080/stats | jq .

# ===== HELP =====

help:
	@echo "Go Microservices Project - Available Commands"
	@echo "=============================================="
	@echo ""
	@echo "Build:"
	@echo "  make build          - Build all services"
	@echo "  make build-payment  - Build payment service only"
	@echo "  make build-order    - Build order service only"
	@echo ""
	@echo "Run:"
	@echo "  make run-payment    - Start Payment service (gRPC :50051)"
	@echo "  make run-order      - Start Order service (HTTP :8080)"
	@echo ""
	@echo "Test:"
	@echo "  make test           - Run all tests"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo ""
	@echo "Development:"
	@echo "  make deps           - Download dependencies"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Lint code"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "Demo (requires services running):"
	@echo "  make demo           - Create a test order"
	@echo "  make demo-list      - List all orders"
	@echo "  make demo-health    - Check service health"
	@echo "  make demo-stats     - Get service statistics"
