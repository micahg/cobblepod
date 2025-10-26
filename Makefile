.PHONY: build run clean test deps fmt vet server worker

# Build the worker (main application)
build-worker:
	go build -o cobblepod-worker cmd/worker/main.go

# Build the HTTP server
build-server:
	go build -o cobblepod-server cmd/http/main.go

# Build all binaries
build: build-worker build-server

# Run the worker
run-worker:
	go run cmd/worker/main.go

# Run the HTTP server
run-server:
	go run cmd/http/main.go

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

# Run server tests
test-server:
	go test ./...

# Run UI tests
test-ui:
	cd ui && npm run test:run

# Run all tests
test: test-server test-ui

# Run all checks
check: fmt vet test-server

# Install the application
install:
	go install ./cmd/http

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o cobblepod-worker-linux-amd64 cmd/worker/main.go
	GOOS=darwin GOARCH=amd64 go build -o cobblepod-worker-darwin-amd64 cmd/worker/main.go
	GOOS=windows GOARCH=amd64 go build -o cobblepod-worker-windows-amd64.exe cmd/worker/main.go
	GOOS=linux GOARCH=amd64 go build -o cobblepod-server-linux-amd64 cmd/http/main.go
	GOOS=darwin GOARCH=amd64 go build -o cobblepod-server-darwin-amd64 cmd/http/main.go
	GOOS=windows GOARCH=amd64 go build -o cobblepod-server-windows-amd64.exe cmd/http/main.go

# Default target
all: clean deps check build

image:
	docker build -t cobblepod:latest .