# Implementation Plan: Agent Hub

## Block 1 — Project Scaffold
- 1a: Backend scaffold (Go module, dirs, main.go skeleton)
- 1b: Frontend scaffold (Vite + Tailwind + shadcn/ui + deps)

## Block 2 — Core Services
- 2a: Config service (JSON config load/save)
- 2b: Database (SQLite, auto-migrate, CRUD)

## Block 3 — Git + Agent
- 3a: Git operations (list projects, branches, worktree, diff)
- 3b: Agent manager (spawn, kill, detect prompt, stdin piping)

## Block 4 — API + WebSocket
- REST handlers + WebSocket with per-session broadcasting

## Block 5 — Frontend Scaffold
- shadcn/ui components, API client hooks, WebSocket hook, routing

## Block 6 — Frontend Pages
- 6a: Dashboard (sessions grouped by status)
- 6b: New Session form (project/agent/branch pickers + task input)
- 6c: Session Detail (chat, live output, code changes, messaging)

## Risks
- Agent cleanup on crash → kill on shutdown, SIGKILL fallback
- ANSI codes in output → strip before store
- Git worktree conflicts → error + user message
