# Phased Execution Plan — flowjs-works

> This plan replaces and extends `docs/planDeTrabajo.md` with a structured Epic → Task breakdown aligned to the full specification.

---

## Phase 0 — Foundation & Contracts (Current)

**Goal**: Establish the knowledge base, DSL contracts, and API specs before writing runtime code.

| # | Task | Status |
|---|------|--------|
| 0.1 | Create `context/` AI knowledge base | ✅ |
| 0.2 | Define full DSL TypeScript types (`dsl.ts`) | ✅ |
| 0.3 | Define full DSL Go models (`process.go`) | ✅ |
| 0.4 | Write OpenAPI spec for Manager API (`openapi.yaml`) | ✅ |
| 0.5 | Document DB schema (config + audit) | ✅ |
| 0.6 | Phased execution plan (this document) | ✅ |
| 0.7 | Technical risk analysis | ✅ |

---

## Phase 1 — Core Engine (Épica: Motor de Ejecución)

**Goal**: A Go binary that parses the full DSL, resolves JSONPath references, executes nodes sequentially, and handles transitions (success/error/condition).

| # | Task | Description |
|---|------|-------------|
| 1.1 | Transition engine | Implement `success`, `error`, `condition`, `nocondition` routing in the executor |
| 1.2 | Secret resolution | Runtime resolver that fetches `secret_ref` from DB/vault and injects into node config |
| 1.3 | Enhanced context | Extend `ExecutionContext` to support nested JSONPath with array indexing |
| 1.4 | Node: SQL activity | Implement `sql` node with Postgres/MySQL/Oracle driver support |
| 1.5 | Node: Mail activity | Implement `mail` node (send + receive via SMTP/IMAP) |
| 1.6 | Node: RabbitMQ activity | Implement `rabbitmq` producer node (AMQP publish) |
| 1.7 | Node: Transform activity | Implement `transform` node (json2csv, xml2json, json2xml) |
| 1.8 | Node: File activity | Implement local `file` node (create, read, delete) |
| 1.9 | Node: Log activity | Enhance existing `logger` to match `log` DSL spec |
| 1.10 | Node: Code activity | Enhance Goja sandbox with proper timeout and memory limits |

---

## Phase 2 — Secrets Module (Épica: Gestión de Secretos)

**Goal**: A transversal secrets store that nodes reference at runtime without embedding credentials.

| # | Task | Description |
|---|------|-------------|
| 2.1 | Secrets DB table | Deploy `secrets` table to config database |
| 2.2 | Secrets CRUD API | REST endpoints: create, list (metadata-only), delete |
| 2.3 | Encryption at rest | AES-256-GCM encryption/decryption helper in Go |
| 2.4 | Engine integration | Inject resolved secrets into node `config` at execution time |
| 2.5 | Audit scrubbing | Ensure secret values are redacted from `input_data`/`output_data` in audit logs |

---

## Phase 3 — Designer UI (Épica: Diseñador Visual)

**Goal**: Extend the React Flow canvas with all node types, the full transition palette, and live debug.

| # | Task | Description |
|---|------|-------------|
| 3.1 | Node palette expansion | Add React Flow components for all 11 node types |
| 3.2 | Trigger palette expansion | Add UI for all 6 trigger types (cron, rest, soap, rabbitmq, mcp, manual) |
| 3.3 | Transition UI | Visual handles for success/error/condition/nocondition edges |
| 3.4 | Secret selector | Dropdown in node config panels to pick a `secret_ref` |
| 3.5 | Serializer update | Update graph → DSL serializer for new types and transition semantics |
| 3.6 | Live debug panel | Execute DSL in debug mode and show node-by-node output inline |

---

## Phase 4 — File-Transfer Nodes (Épica: Conectividad Ficheros)

**Goal**: Implement SFTP, S3, and SMB nodes for bidirectional file transfer.

| # | Task | Description |
|---|------|-------------|
| 4.1 | Node: SFTP activity | Go SFTP client with get/put, regex filter, folder creation |
| 4.2 | Node: S3 activity | AWS SDK integration for bucket operations |
| 4.3 | Node: SMB activity | SMB2/3 client for Windows share operations |
| 4.4 | Designer components | Config panels and icons for SFTP, S3, SMB |

---

## Phase 5 — Triggers & Deployment (Épica: Despliegues)

**Goal**: Full trigger lifecycle — cron scheduler, RabbitMQ consumer, MCP listener — and deploy/stop management.

| # | Task | Description |
|---|------|-------------|
| 5.1 | Cron trigger | In-process cron scheduler (robfig/cron) |
| 5.2 | RabbitMQ trigger | AMQP consumer that feeds flow execution |
| 5.3 | MCP trigger | Model Context Protocol listener |
| 5.4 | Deploy/Stop API | Manager endpoints to deploy (start triggers) and stop flows |
| 5.5 | Process status tracking | Persist deploy state (draft/deployed/stopped) in config DB |

---

## Phase 6 — Observability & Replay (Épica: Auditoría Avanzada)

**Goal**: Execution history viewer, log search, partial replay.

| # | Task | Description |
|---|------|-------------|
| 6.1 | Execution list UI | Paginated table of executions with status filters |
| 6.2 | Activity log viewer | Step-by-step payload inspector per execution |
| 6.3 | Full replay | Re-execute a flow with the original trigger data |
| 6.4 | Partial replay | Resume from a specific node using its persisted `input_data` |
| 6.5 | Log search | JSONB deep search across `input_data`/`output_data` |

---

## Phase 7 — Cloud-Native & Scale (Épica: Infraestructura)

**Goal**: Kubernetes orchestration, horizontal scaling, and production hardening.

| # | Task | Description |
|---|------|-------------|
| 7.1 | Dockerize all services | Multi-stage Docker builds for engine, audit-logger, designer |
| 7.2 | Kubernetes manifests | Deployments, services, config maps, secrets for K8s |
| 7.3 | Per-flow isolation | Orchestrator that spins a dedicated pod per deployed flow |
| 7.4 | Health checks & metrics | Prometheus metrics, liveness/readiness probes |
| 7.5 | Hot-reload | Rolling update of flow DSL without message loss |
