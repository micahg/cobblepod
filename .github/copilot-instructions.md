# Project Structure

- `ui` contains the user interface (react frontend)
- `server` contains the backend (go server and worker)

# General Guidelines

- Do not create documentation unless you are asked.

# Testing

- `ui` should run with `npm run test:run` (or `make test-ui` from the top level)to avoid waiting for file changes
- `server` should run with `go test ./...` (or `make test-server` from the top level) to run all tests