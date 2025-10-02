# Project Structure

- `ui` contains the user interface (react frontend)
- `server` contains the backend (go server and worker)

# General Guidelines

- Do not create a README.md files subdirectories.

# Testing

- `ui` should run with `npm run test:run` to avoid waiting for file changes
- `server` should run with `make test` to run all tests