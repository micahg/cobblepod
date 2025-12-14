.PHONY: build run clean test deps fmt vet server worker

# Build the worker (main application)
build-worker:
	go build -o cobblepod-worker cmd/worker/main.go

# Build the HTTP server
build-server:
	go build -o cobblepod-server cmd/server/main.go

# Build all binaries
build: build-worker build-server

# Run the worker
run-worker:
	go run cmd/worker/main.go

# Run the HTTP server
run-server:
	env $(cat .env.local | grep -v "^\#") go run cmd/server/main.go

run-docker:
	set -a && . ./.env.local && . ./ui/.env.local && set +a && docker compose up --build

# Clean build artifacts
clean:
	rm -f cobblepod-worker cobblepod-server

# Download dependencies
deps:
	go mod tidy
	go mod download

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run server tests (unit tests only)
test-server:
	go test ./...

# Run UI tests
test-ui:
	cd ui && npm run test:run

# Run all tests
test: test-server test-ui

# Run server integration tests with a dedicated Valkey instance
# This runs BOTH unit and integration tests, providing combined coverage
test-server-integration:
	@echo "Starting test Valkey instance..."
	@docker run -d -p 6380:6379 --name cobblepod-test-valkey valkey/valkey:latest > /dev/null
	@echo "Waiting for Valkey to be ready..."
	@sleep 2
	@echo "Running integration tests..."
	@VALKEY_PORT=6380 go test -tags integration -coverprofile=coverage.out -v ./...; \
	EXIT_CODE=$$?; \
	echo "Cleaning up..."; \
	docker rm -f cobblepod-test-valkey > /dev/null; \
	exit $$EXIT_CODE
test: test-server test-ui

# Run all checks
check: fmt vet test-server

# Install the application
install:
	go install ./cmd/server

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o cobblepod-worker-linux-amd64 cmd/worker/main.go
	GOOS=darwin GOARCH=amd64 go build -o cobblepod-worker-darwin-amd64 cmd/worker/main.go
	GOOS=windows GOARCH=amd64 go build -o cobblepod-worker-windows-amd64.exe cmd/worker/main.go
	GOOS=linux GOARCH=amd64 go build -o cobblepod-server-linux-amd64 cmd/server/main.go
	GOOS=darwin GOARCH=amd64 go build -o cobblepod-server-darwin-amd64 cmd/server/main.go
	GOOS=windows GOARCH=amd64 go build -o cobblepod-server-windows-amd64.exe cmd/server/main.go

# Default target
all: clean deps check build

image:
	docker build -t cobblepod:latest .