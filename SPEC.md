# Spec: Agent Hub — Agent Session Manager

## Objective

A local web tool for managing concurrent agent sessions (Claude Code and OpenCode). The user can start, monitor, interact with, and review sessions — all from a modern web UI.

**User story:** "I want a dashboard showing all my agent sessions grouped by state. I can start new agent sessions for any project, send them tasks, watch their progress, review their chat history and code changes, and send follow-up messages — all from one UI."

**Success criteria:**
- User can see all sessions grouped by "running", "needs attention", "done"
- User can start a new session: pick agent type, project, write task description, pick base branch
- Starting a session creates a git worktree + feature branch, then spawns the agent
- Agent stdout/stderr is streamed live to the UI
- User can send messages to the agent and see responses in real-time
- Clicking a session shows full chat history + code changes (git diff log)
- Sessions persist across app restarts (SQLite)
- Agent state transitions: "running" ↔ "needs attention" (auto-detect + manual) → "done"

## Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| Backend | Go 1.23+ (Chi router, gorilla/websocket) | Process management, git ops, minimal deps |
| Frontend | React 19 + Vite + TypeScript | Modern, fast dev experience |
| UI Kit | Tailwind CSS v4 + shadcn/ui | Polished, dark-theme components |
| Data | SQLite (mattn/go-sqlite3) | Zero-config, persists locally |
| Real-time | WebSocket | Live streaming agent output |
| HTTP client | TanStack Query | Data fetching, caching, mutations |

## Commands

```
Build (backend):  cd backend && go build -o agent-hub ./cmd/server
Build (frontend): cd frontend && npm run build
Dev (backend):    cd backend && go run ./cmd/server
Dev (frontend):   cd frontend && npm run dev
Dev (both):       cd backend && go run ./cmd/server   # serves frontend build + API
Test (backend):   cd backend && go test ./...
Test (frontend):  cd frontend && npm run test
Lint (backend):   cd backend && golangci-lint run
Lint (frontend):  cd frontend && npm run lint
DB migration:     auto-migrated on startup (no manual migration step)
```

## Project Structure

```
agent-hub/
├── backend/
│   ├── cmd/server/        → Main binary entrypoint
│   ├── internal/
│   │   ├── api/           → HTTP handlers + WebSocket handler
│   │   ├── agent/         → Agent process lifecycle (spawn, kill, stdin)
│   │   ├── git/           → Git worktree, branch creation, diff retrieval
│   │   ├── db/            → SQLite models and queries
│   │   ├── config/        → Config file loading (JSON/YAML)
│   │   └── types/         → Shared types
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── components/    → React components (shadcn/ui + custom)
│   │   ├── pages/         → Route pages (Dashboard, SessionDetail)
│   │   ├── hooks/         → Custom hooks (useWebSocket, useSessions)
│   │   ├── lib/           → Utilities, API client
│   │   └── types/         → TypeScript types
│   ├── package.json
│   ├── vite.config.ts
│   └── tailwind.config.ts
├── config.json            → User config (projects dir, etc.)
├── agent-hub.db           → SQLite database (gitignored)
└── SPEC.md
```

## Code Style

### Go

```go
// handler.go
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
    sessions, err := h.store.ListSessions(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list sessions")
        return
    }
    respondJSON(w, http.StatusOK, sessions)
}
```

- Standard Go layout (`cmd/`, `internal/`)
- No global state; use dependency injection via handler structs
- Error wrapping with `fmt.Errorf("...: %w")`
- Context propagation on all DB/process calls
- SQLite queries via raw SQL + `database/sql` (no ORM)

### TypeScript / React

```tsx
// SessionCard.tsx
export function SessionCard({ session }: { session: Session }) {
  return (
    <div className="rounded-lg border bg-card p-4">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">{session.projectName}</h3>
        <StatusBadge status={session.status} />
      </div>
      <p className="text-sm text-muted-foreground mt-1">{session.taskDescription}</p>
    </div>
  )
}
```

- Functional components with hooks (no class components)
- TypeScript strict mode
- Tailwind utility classes + shadcn/ui `cn()` helper for conditional classes
- No prop drilling — use TanStack Query for server state, React context for UI state
- File names: PascalCase for components, camelCase for utilities

## Database Schema (SQLite)

```sql
CREATE TABLE sessions (
    id            TEXT PRIMARY KEY,
    agent_type    TEXT NOT NULL,        -- 'opencode' | 'claude-code'
    project_path  TEXT NOT NULL,
    project_name  TEXT NOT NULL,
    base_branch   TEXT NOT NULL,
    feature_branch TEXT NOT NULL,
    worktree_path TEXT NOT NULL,
    task_description TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'running',  -- 'running' | 'needs_attention' | 'done'
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    exited_at     DATETIME
);

CREATE TABLE messages (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role       TEXT NOT NULL,           -- 'user' | 'agent'
    content    TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE code_snapshots (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    commit_hash TEXT,
    diff       TEXT NOT NULL,
    summary    TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## API Endpoints

| Method | Path | Description |
|---|---|---|
| GET | /api/sessions | List all sessions |
| POST | /api/sessions | Create + start a new session |
| GET | /api/sessions/:id | Get session details |
| PATCH | /api/sessions/:id/status | Update session status |
| POST | /api/sessions/:id/message | Send message to agent |
| GET | /api/sessions/:id/messages | Get chat history |
| GET | /api/sessions/:id/changes | Get code changes |
| POST | /api/sessions/:id/stop | Kill agent process |
| GET | /api/projects | List projects in configured directory |
| GET | /api/projects/:name/branches | List git branches for a project |
| GET | /ws/sessions/:id | WebSocket — stream agent output |
| GET | /api/config | Get current config |
| PUT | /api/config | Update config |

## WebSocket Protocol

Messages are JSON-encoded:

```json
// Server → Client (agent output)
{ "type": "output", "data": "Checking dependencies..." }

// Server → Client (status change)
{ "type": "status", "status": "needs_attention" }

// Server → Client (error)
{ "type": "error", "message": "process exited with code 1" }

// Client → Server (send message to agent)
{ "type": "message", "content": "fix the type error" }
```

## Agent State Machine

```
                  spawn + initial message
                      │
                      ▼
                 ┌──────────┐
         ┌──────│  running  │◄──────────────┐
         │      └─────┬────┘                │
         │            │ agent outputs        │ user sends
         │            │ prompt pattern       │ new message
         │            ▼                      │
         │      ┌──────────────┐             │
         │      │needs_attention│────────────┘
         │      └──────┬───────┘
         │             │ user clicks "done"
         │             ▼
         │      ┌──────────┐
         └──────│   done   │
                └──────────┘
```

- **Auto-detect "needs attention":** Agent output ends with a prompt pattern (trailing `>` or `───` separator indicating it's waiting for input)
- **Manual override:** User can click a button to flip between states
- **Done:** Agent process exits, or user manually marks done

## Config File (`config.json`)

```json
{
  "projects_dir": "/Users/marvinknapp/Documents/projekte",
  "agents": {
    "opencode": "/Users/marvinknapp/.local/bin/opencode",
    "claude-code": "claude"
  }
}
```

## Testing Strategy

| Layer | Framework | Location | Coverage Target |
|---|---|---|---|
| Go unit tests | `testing` + `httptest` | `backend/internal/...` | 80%+ |
| Go integration | `testing` with SQLite | `backend/internal/db/` | Key paths |
| React unit | Vitest + testing-library | `frontend/src/` alongside components | 70%+ |
| E2E (future) | Playwright | `e2e/` | Critical paths |

Critical test paths:
- Agent process spawn and stdin/stdout piping
- State transitions (running → needs_attention → done)
- Git worktree creation and branch checkout
- WebSocket message streaming
- Session persistence across restart

## Boundaries

**Always do:**
- Run `go vet ./...` and `npm run lint` before commits
- Stream agent output to UI in real-time (never buffer entire output)
- Validate all API inputs
- Sanitize branch names (alphanumeric + hyphens only)
- Kill agent processes on session stop / app shutdown
- Use context cancellation to clean up goroutines

**Ask first:**
- Adding new dependencies
- Changing the database schema
- Modifying the WebSocket protocol
- Adding authentication (future consideration)
- Changing project directory structure

**Never do:**
- Commit secrets or API keys
- Store agent API keys in the database
- Run agent processes as root
- Remove or modify existing agent sessions without confirmation
- Delete git worktrees without user confirmation
- Hardcode user paths

## Open Questions

- [RESOLVED] Agent interaction: subprocess stdin/stdout
- [RESOLVED] Chat history: captured from stdout/stderr
- [RESOLVED] Needs-attention detection: auto-detect prompt pattern + manual toggle
- [RESOLVED] Branch naming: user types description → auto-generate branch name
- Should agent output include ANSI color codes stripped before storing/comparing?
- What's the chunking strategy for large git diffs in the UI?
