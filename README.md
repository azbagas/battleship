# Battleship

A browser-based, real-time, two-player online Battleship game built as a React/TypeScript SPA frontend with a server-authoritative Go + SQLite backend over WebSockets.

## Documentation
- [PRD (Product Requirements)](docs/PRD.md): Scope, user flows, and success criteria.
- [Architecture](docs/ARCHITECTURE.md): Technical architecture, data model, and invariants.
- [Protocol Specification](docs/PROTOCOL.md): WebSocket wire protocol, state transitions, message schemas, and error codes.
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
The backend runs zero-config with sensible defaults out of the box. Key environment variables (full list in [`backend/.env.example`](backend/.env.example)):
- `PORT`: HTTP/WebSocket listening port (default: `8080`).
- `LOG_LEVEL` / `LOG_FORMAT`: Structured logging level (`INFO`, `DEBUG`, `WARN`, `ERROR`) and format (`text`, `json`).
- `DB_PATH`: SQLite database file path for finished game records (default: `battleship.db`).
- `ALLOWED_ORIGINS`: Comma-separated list of allowed WebSocket origins (default: `*` in development).
- `MAX_PAYLOAD_BYTES`: Maximum incoming message payload size (default: `32768` = 32 KB).
- `MAX_CONNS_PER_IP`: Max concurrent WebSocket connections per IP address (default: `10`).
- `RATE_LIMIT_MESSAGES_PER_SEC` / `RATE_LIMIT_BURST`: Per-connection token bucket rate limiter (defaults: `10.0` msg/s, `20.0` burst).

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

