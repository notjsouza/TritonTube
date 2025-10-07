.PHONY: all
all: proto

# Generate Protocol Buffer files
.PHONY: proto
proto:
	protoc --go_out=. --go-grpc_out=. proto/*.proto

# Install frontend dependencies
.PHONY: install
install:
	npm install

# Start all services (requires PowerShell)
.PHONY: dev
dev:
	powershell -ExecutionPolicy Bypass -File start-dev.ps1

# Stop all services
.PHONY: stop
stop:
	powershell -ExecutionPolicy Bypass -File scripts/stop-dev.ps1

# Build frontend for production
.PHONY: build-frontend
build-frontend:
	npm run build

# Build backend
.PHONY: build-backend
build-backend:
	go build -o bin/web.exe ./cmd/web
	go build -o bin/storage.exe ./cmd/storage
	go build -o bin/admin.exe ./cmd/admin

# Build everything
.PHONY: build
build: build-frontend build-backend

# Run tests
.PHONY: test
test:
	go test ./...
	npm test -- --watchAll=false

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf build/
	rm -rf bin/
	rm -rf node_modules/
	rm -f metadata.db

# Format code
.PHONY: fmt
fmt:
	go fmt ./...
	npm run format 2>/dev/null || echo "No format script defined"

# Lint code
.PHONY: lint
lint:
	go vet ./...
	npm run lint 2>/dev/null || echo "No lint script defined"

# Setup development environment
.PHONY: setup
setup: install
	@echo "Creating storage directories..."
	@mkdir -p storage/8090 storage/8091 storage/8092
	@echo "Setup complete! Run 'make dev' to start all services"

# Quick start - setup and run
.PHONY: start
start: setup dev

# Show help
.PHONY: help
help:
	@echo "TritonTube Makefile Commands:"
	@echo ""
	@echo "  make install          - Install frontend dependencies"
	@echo "  make dev              - Start all services (frontend + backend)"
	@echo "  make stop             - Stop all running services"
	@echo "  make build            - Build frontend and backend for production"
	@echo "  make build-frontend   - Build only the frontend"
	@echo "  make build-backend    - Build only the backend"
	@echo "  make test             - Run all tests"
	@echo "  make clean            - Remove build artifacts"
	@echo "  make fmt              - Format all code"
	@echo "  make lint             - Lint all code"
	@echo "  make setup            - Setup development environment"
	@echo "  make start            - Setup and start all services"
	@echo "  make proto            - Generate Protocol Buffer files"
	@echo "  make help             - Show this help message"
	@echo ""
