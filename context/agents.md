# 🤖 AI Agent Onboarding — flowjs-works

> Extended architecture rules and conventions for AI agents. For the short-form checklist see the root `AGENTS.md`.

## 1. System Overview

**flowjs-works** is an iPaaS no-code/low-code platform that replaces legacy XML-based integration tools (TIBCO BW) with a lightweight microservices architecture driven by JSON.

### Architecture Planes

| Plane | Role | Technology |
|-------|------|------------|
| **Control Plane** | Design, management, deployment | React UI + Manager API |
| **Data Plane** | Flow execution (one microservice per flow) | Go Runtime Engine |
| **Persistence** | Audit & config storage | PostgreSQL (JSONB) |
| **Messaging** | Async audit events | NATS |

### Key Repositories Layout

```
/apps/designer/          → React + TypeScript + Tailwind (Vite)
/services/engine/        → Go execution engine (Goja for JS)
/services/audit-logger/  → Go audit consumer (NATS → PostgreSQL)
/init-db/                → PostgreSQL initialization scripts
/context/                → AI knowledge base (this folder)
/docs/                   → Architecture & planning documents
```

## 2. DSL Contract Rules

The JSON DSL is the **single source of truth** for a process. Every agent must respect these invariants:

1. **One Trigger per Process** — exactly one entry point.
2. **N Nodes** — zero or more activity nodes.
3. **Transitions connect nodes** — every non-trigger node must have at least one inbound transition.
4. **JSONPath references** — all data references use `$.trigger.*` or `$.nodes.<id>.output.*` syntax.
5. **Secret references** — credentials are never inline; use `secret_ref` pointing to the Secrets store.
6. **Type discriminators** — `trigger.type` and `node.type` determine the config shape. Use TypeScript discriminated unions.

### Canonical Type Sources

- TypeScript: `apps/designer/src/types/dsl.ts`
- Go: `services/engine/internal/models/process.go`
- Example JSON: `docs/dsl.json`

## 3. Coding Standards

### Go (Engine & Audit Logger)

- **Error handling**: Always check `if err != nil`. Never silently discard errors.
- **Structure**: `/cmd` for entry points, `/internal` for private logic.
- **Concurrency**: Activities must be thread-safe. Use channels for coordination.
- **Linter**: Code must pass `golangci-lint`. Max cyclomatic complexity: 10.
- **Audit emission**: Every completed activity emits an async NATS event on `audit.logs`.
- **Testing**: Use `testify/assert` and `testify/require`. Mock all external calls (HTTP, DB, NATS).

### TypeScript (Designer UI)

- **Framework**: React 18+ with `@xyflow/react` (React Flow) for the canvas.
- **Styling**: Tailwind CSS.
- **Strict types**: `any` is forbidden. Use discriminated unions for node configs.
- **State serialization**: The React Flow graph must always be serializable to JSON DSL.
- **Testing**: Vitest with testing-library.

## 4. Node Implementation Checklist

When implementing a new activity node, ensure:

- [ ] TypeScript interface added to `dsl.ts` (config + output shape).
- [ ] Go struct added to `process.go` (matching JSON tags).
- [ ] Activity handler registered in `activities/activity.go`.
- [ ] `input_mapping` resolution works for the new node.
- [ ] Unit tests cover: success, error, malformed JSON, timeout, missing context ref.
- [ ] NATS audit event emitted on completion.
- [ ] React Flow component added to `apps/designer/src/components/`.
- [ ] Palette entry added in `designer.ts`.

## 5. Secret Management Rules

- Secrets are stored in a centralized vault (DB-backed initially, Vault/KMS later).
- Nodes reference secrets via `secret_ref: "<secret_id>"` in their config.
- The engine resolves `secret_ref` at runtime, never at design time.
- Secrets must **never** appear in audit logs (`input_data` / `output_data`).

## 6. Transition Semantics

| Type | Condition | When |
|------|-----------|------|
| `success` | (none) | Previous node completed without error |
| `error` | (none) | Previous node threw an error (visual try/catch) |
| `condition` | JSONPath expression | Expression evaluates to truthy |
| `nocondition` | (none) | Else branch — only valid when a `condition` also exits the same node |

## 7. Replay & Idempotency

- Activities should be idempotent whenever possible.
- `input_data` and `output_data` are persisted in the Audit DB for replay.
- Full Replay: restart from trigger. Partial Replay: restart from a specific node using its last persisted input.

## 8. Naming Conventions

| Entity | Convention | Example |
|--------|-----------|---------|
| Process IDs | `kebab-case` | `user-onboarding-flow` |
| Node IDs | `snake_case` | `send_welcome_email` |
| Trigger IDs | `trg_` prefix | `trg_01` |
| Secret IDs | `sec_` prefix | `sec_postgres_main` |
| Go packages | lowercase, short | `activities`, `engine`, `models` |
| TS files | `camelCase.ts` | `dsl.ts`, `serializer.ts` |
