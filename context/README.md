# 📚 Knowledge Base — flowjs-works

This folder serves as the **AI Onboarding Knowledge Base** for the flowjs-works platform. Every AI agent contributing to this project should read these files before generating code.

## Contents

| File | Purpose |
|------|---------|
| `agents.md` | Architecture rules, coding standards, and conventions for AI agents |
| `db_schema.sql` | Complete PostgreSQL schema (config + audit databases) |
| `openapi.yaml` | OpenAPI 3.0 specification for the Manager API (Control Plane) |
| `dsl-reference.md` | Quick-reference guide for the JSON DSL contract |

## How to Use

1. **Before coding**: Read `agents.md` for project conventions.
2. **Before DB work**: Consult `db_schema.sql` for the canonical schema.
3. **Before API work**: Consult `openapi.yaml` for endpoint contracts.
4. **Before DSL work**: Review the TypeScript types in `apps/designer/src/types/dsl.ts` and the Go models in `services/engine/internal/models/process.go`.

## Canonical Type Sources

- **TypeScript DSL**: `apps/designer/src/types/dsl.ts`
- **Go DSL Models**: `services/engine/internal/models/process.go`
- **Audit Types**: `apps/designer/src/types/audit.ts`
- **DB Schema**: `init-db/02-audit-schema.sql` (deployed), this folder's `db_schema.sql` (reference)
