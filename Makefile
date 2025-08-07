.PHONY: build run clean test deps fmt vet

# Build the application
build:
	go build -o cobblepod main.go

# Run the application
run:
	go run main.go

# Clean build artifacts
clean:
	rm -f cobblepod

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

# Run tests
test:
	go test ./...

# Run all checks
check: fmt vet test

# Install the application
install:
	go install

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o cobblepod-linux-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -o cobblepod-darwin-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -o cobblepod-windows-amd64.exe main.go

# Default target
all: clean deps check build
