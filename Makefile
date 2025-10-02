.PHONY: build run clean test deps fmt vet http-server

# Build the application
build:
	cd server && go build -o ../cobblepod main.go

# Build the HTTP server
build-http:
	cd server && go build -o ../cobblepod-http cmd/http/main.go

# Run the application
run:
	cd server && go run main.go

# Run the HTTP server
run-http:
	cd server && go run cmd/http/main.go

# Clean build artifacts
clean:
	rm -f cobblepod cobblepod-http

# Download dependencies
deps:
	cd server && go mod tidy
	cd server && go mod download

# Format code
fmt:
	cd server && go fmt ./...

# Run go vet
vet:
	cd server && go vet ./...

# Run server tests
test-server:
	cd server && go test ./...

# Run UI tests
test-ui:
	cd ui && npm run test:run

# Run all tests
test: test-server test-ui

# Run all checks
check: fmt vet test-server

# Install the application
install:
	cd server && go install

# Build for multiple platforms
build-all:
	cd server && GOOS=linux GOARCH=amd64 go build -o ../cobblepod-linux-amd64 main.go
	cd server && GOOS=darwin GOARCH=amd64 go build -o ../cobblepod-darwin-amd64 main.go
	cd server && GOOS=windows GOARCH=amd64 go build -o ../cobblepod-windows-amd64.exe main.go

# Default target
all: clean deps check build

image:
	docker build -t cobblepod:latest ./server