# Agent Hub

## Quick start

```sh
# Backend (Go 1.25+)
cd backend && go run ./cmd/server              # serves API on :8080 + frontend dist

# Frontend dev (needs backend running)
cd frontend && npm run dev                      # Vite on :5173, proxies /api + /ws to :8080

# Full production build
cd frontend && npm run build                    # tsc -b && vite build → frontend/dist/
# then serve with `go run ./cmd/server` from backend/
```

## Key commands

| What | How |
|---|---|
| Backend build | `cd backend && go build -o agent-hub ./cmd/server` |
| Frontend build | `cd frontend && npm run build` |
| Frontend lint | `cd frontend && npm run lint` |
| Backend test | `cd backend && go test ./...` (no tests exist yet) |
| Frontend test | `cd frontend && npm run test` (no tests exist yet) |

## Architecture

- **Monorepo**: `backend/` (Go) + `frontend/` (React/Vite). `config.json` at root.
- **DB**: SQLite auto-migrated on startup at `agent-hub.db`. WAL mode. No manual migrations.
- **Config**: `config.json` auto-created with defaults if missing. Override with `AGENT_HUB_CONFIG` env. Port override with `AGENT_HUB_PORT`.
- **API**: Go 1.22+ `http.ServeMux` with method-based routing (`GET /api/sessions`, `POST /api/sessions`, `PATCH .../status`, etc.). No Chi/gorilla/mux router.
- **WebSocket**: `gorilla/websocket`, path pattern `GET /ws/sessions/{id}`.
- **Frontend**: React 19, TypeScript 6, Tailwind CSS v4 (custom dark theme vars in `index.css`), shadcn/ui components in `frontend/src/components/ui/`, TanStack Query, React Router v7.
- **SPA fallback**: Backend rewrites paths without dots to `/` for the SPA router.
- **Entrypoints**: `backend/cmd/server/main.go` · `frontend/src/main.tsx`

## Conventions

- Go: standard `cmd/` `internal/` layout, DI via handler structs, raw SQL via `database/sql`, `fmt.Errorf("...: %w")` error wrapping.
- Frontend: PascalCase component files, `camelCase` utilities, `@/` path alias for `src/`, `cn()` from `tailwind-merge` for conditional classes.
- Branch naming: `agent/<sanitized-description>` (lowercase, hyphens, max 80 chars).
- Session states: `running` → `needs_attention` (auto-detected from prompt patterns) → `done`.
- Go deps only: `uuid`, `gorilla/websocket`, `mattn/go-sqlite3`.

## Gotchas

- **Go 1.25+ required** for method-based routing (`GET /api/sessions` syntax).
- **No Go linter config** exists (no `.golangci.yml`). Run `go vet ./...` manually.
- **`config.json` gitignored** — contains local user paths. Copy template from `SPEC.md`.
- **No tests exist yet** in either backend or frontend.
- Frontend TypeScript config uses project references (`tsconfig.json` → `tsconfig.app.json` + `tsconfig.node.json`).
- WebSocket URL hardcodes `localhost:8080` when origin is localhost.
- ANSI codes stripped from agent output before storing/broadcasting.
- Worktrees created at `../<feature-branch>` relative to project.
- **OpenCode sessions use HTTP API**: Agent Hub spawns `opencode serve` in each worktree and communicates via REST + SSE (no subprocess piping). Claude Code still uses the subprocess approach.
- **Port range**: OpenCode servers use ports 14100–14200 by default; configure via `opencode_server.port_range` in `config.json`.
- **OpenCode binary required**: The `opencode` binary must be on `$PATH` (or configured in `agents.opencode`).
- **External OpenCode server**: Set `opencode_server.url` in `config.json` (e.g. `"http://localhost:4096"`) to use an existing `opencode serve` instead of spawning per-session servers.
- **Reconnect after restart**: If the backend restarts, messages sent to orphaned OpenCode sessions will reconnect to the configured server URL and deliver the message synchronously. The external session ID is persisted in the database.

## References

- `SPEC.md` — canonical spec (API endpoints, schema, state machine, WebSocket protocol)
- `PLAN.md` — implementation blocks (Blocks 1-6, not all complete)
- `.opencode/skills/` — OpenCode skill definitions for the project
