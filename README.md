# Battleship

A browser-based, real-time, two-player online Battleship game built as a React/TypeScript SPA frontend with a server-authoritative Go + SQLite backend over WebSockets.

## Documentation
- [PRD (Product Requirements)](docs/PRD.md): Scope, user flows, and success criteria.
- [Architecture](docs/ARCHITECTURE.md): Technical architecture, data model, and invariants.
- [Tasks](docs/TASKS.md): Current development plan and task progress.

## Prerequisites
- [Go](https://go.dev/) 1.23+
- [Node.js](https://nodejs.org/) 22+ & npm
- [Docker](https://www.docker.com/) & Docker Compose (optional for local containerized run)

## Quick Start

### Install
```bash
npm run install:all
# or install individually:
# npm install && cd frontend && npm install && cd ../backend && go mod download
```

### Run Locally
Run both backend and frontend concurrently with a single command:
```bash
npm run dev
```

Or run them in separate terminals if preferred:
```bash
# Backend:
cd backend
go run ./cmd/server

# Frontend:
cd frontend
npm run dev
```

### Run with Docker Compose
```bash
docker compose up --build
```
Access frontend at `http://localhost:3000` and backend health check at `http://localhost:8080/healthz`.

## Configuration
The backend runs zero-config with sensible defaults out of the box. Optional environment variables (`PORT`, `LOG_LEVEL`, `LOG_FORMAT`, and upcoming database/security knobs) are documented in [`backend/.env.example`](backend/.env.example).

### Test
```bash
cd backend && go test -race ./... && cd ../frontend && npm test
```

### Lint
```bash
cd backend && golangci-lint run && cd ../frontend && npm run lint
```

### Build
```bash
cd backend && go build ./cmd/server && cd ../frontend && npm run build
```

