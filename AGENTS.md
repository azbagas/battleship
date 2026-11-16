# AGENTS.md

<!-- The collaboration contract. Keep it short and operational.
     Each rule appears once. Product details go in docs/PRD.md, technical design in docs/ARCHITECTURE.md. -->

## Read First
- `docs/PRD.md`: what we are building (source of truth for scope).
- `docs/ARCHITECTURE.md`: how it is built, including the stack. Read before changing structure, data, or dependencies.
- `docs/TASKS.md`: what to work on now.
- `README.md`: human-facing setup and usage.

## Project Summary
A browser-based, real-time, two-player online Battleship game built as a React/TypeScript SPA frontend with a server-authoritative Go + SQLite backend over WebSockets.

## Commands
```
Install:  npm run install:all        # or cd frontend && npm install && cd ../backend && go mod download
Run dev:  npm run dev                # or docker compose up --build
Test:     cd backend && go test -race ./... && cd ../frontend && npm test
E2E:      npm run test:e2e
Lint:     cd backend && golangci-lint run && cd ../frontend && npm run lint
Build:    cd backend && go build ./cmd/server && cd ../frontend && npm run build
```

## Working Rules

### Scope
- docs/PRD.md is the source of truth. Never build anything that is not in it.
- For anything new, propose a PRD update first and wait for approval. Small tweaks (labels, spacing, copy) don't need one.
- Ask when requirements are ambiguous instead of guessing.

### Workflow
- Work on ONE task at a time from docs/TASKS.md, in order.
- If a task touches more than a couple of files, state a brief plan first.
- Mark a task complete only when its "Done when" is verified (tests pass, lint clean, build succeeds).
- Found missing or extra work? Add a new task. Never do it silently.

### Code
- Keep code simple. Follow existing patterns and prefer existing libraries over custom code.
- Write or update tests for new behavior.
- Don't add dependencies or delete large areas of code without asking.
- Never commit secrets, API keys, or `.env` files.

## Keeping Docs Current
Update docs in the same change as the code. Describe the current state, not history. At the end of each task, say which docs you updated, or "no doc updates needed". If docs contradict each other or the code, flag it and fix it (or ask which is correct).

- **docs/TASKS.md:** every task. Tick boxes, refresh "Current focus", add discovered tasks, record non-obvious decisions in Notes, archive finished versions.
- **docs/ARCHITECTURE.md:** when the stack, structure, data model, data flow, services, or dependencies change. Log significant decisions under Key Decisions.
- **docs/PRD.md:** only with approval. Draft the change, show it, wait. Clarifications that don't change behavior can be made directly. No code or schemas. Never reuse or renumber feature IDs.
- **README.md:** when install, commands, environment variables, or usage change. Keep it short and for humans; link to docs/ instead of copying.
- **AGENTS.md:** update Commands and Conventions directly. Never add, remove, or loosen Working Rules without approval. Keep it short and don't duplicate other docs.

## Conventions
- Naming: Go uses standard `MixedCaps` / `camelCase` with short, lowercase package names (`game`, `lobby`, `ws`, `store`); React components use `PascalCase.tsx` and hooks/utilities use `camelCase.ts`; WebSocket message types use `domain.action` dot notation (e.g., `lobby.create`, `battle.fire`).
- Go idioms: Prefer tagged switch statements over `if`/`else if` chains when comparing a single variable against multiple values (avoids staticcheck `QF1003`); ensure code remains clean of editor/linter warnings.
- Error handling: Go functions return explicit `error` values wrapped with context (`fmt.Errorf("...: %w", err)`); malformed or out-of-turn WebSocket messages must never panic or crash the process—reject them with a structured `{ "type": "error", "payload": { "code", "message" } }` message.
- Comments: Document exported Go symbols and non-obvious state transitions or concurrency invariants; explain *why* rather than restating *what* the code does.
- UI/UX: Follow guidelines in `.agents/skills/frontend-design` and `.agents/skills/ui-ux-pro-max` when styling and building frontend screens or components.

## Definition of Done
Behavior matches the PRD "Done when", tests pass, lint is clean, the diff has no unrelated changes, and affected docs are updated.
