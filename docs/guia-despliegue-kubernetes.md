# FlowJS-Works — Guía de Implementación y Despliegue en Kubernetes

> **Idioma:** Español  
> **Versión:** 1.0.0  
> **Última actualización:** Marzo 2026

---

## Tabla de contenidos

1. [Resumen del Proyecto](#1-resumen-del-proyecto)
2. [Arquitectura General](#2-arquitectura-general)
3. [Resumen de lo Implementado](#3-resumen-de-lo-implementado)
   - [Fase 0 — Fundación y Contratos](#fase-0--fundación-y-contratos)
   - [Fase 1 — Motor de Ejecución (Engine)](#fase-1--motor-de-ejecución-engine)
   - [Fase 2 — Módulo de Secretos](#fase-2--módulo-de-secretos)
   - [Fase 3 — Diseñador Visual (Designer UI)](#fase-3--diseñador-visual-designer-ui)
   - [Fase 4 — Nodos de Transferencia de Ficheros](#fase-4--nodos-de-transferencia-de-ficheros)
   - [Fase 5 — Triggers y Despliegues](#fase-5--triggers-y-despliegues)
   - [Fase 6 — Observabilidad y Replay](#fase-6--observabilidad-y-replay)
   - [Fase 7 — Cloud-Native y Escala](#fase-7--cloud-native-y-escala)
4. [Catálogo de Actividades](#4-catálogo-de-actividades)
5. [Catálogo de Triggers](#5-catálogo-de-triggers)
6. [API REST del Engine](#6-api-rest-del-engine)
7. [Endpoints del Audit Logger](#7-endpoints-del-audit-logger)
8. [Guía de Despliegue en Kubernetes](#8-guía-de-despliegue-en-kubernetes)
   - [Prerrequisitos](#81-prerrequisitos)
   - [Paso 1 — Clonar el repositorio](#paso-1--clonar-el-repositorio)
   - [Paso 2 — Construir las imágenes Docker](#paso-2--construir-las-imágenes-docker)
   - [Paso 3 — Publicar las imágenes en un registro](#paso-3--publicar-las-imágenes-en-un-registro)
   - [Paso 4 — Configurar secretos de producción](#paso-4--configurar-secretos-de-producción)
   - [Paso 5 — Inicializar las bases de datos](#paso-5--inicializar-las-bases-de-datos)
   - [Paso 6 — Aplicar los manifiestos Kubernetes](#paso-6--aplicar-los-manifiestos-kubernetes)
   - [Paso 7 — Verificar el estado del despliegue](#paso-7--verificar-el-estado-del-despliegue)
   - [Paso 8 — Exponer la UI al exterior](#paso-8--exponer-la-ui-al-exterior)
   - [Paso 9 — Verificar health checks y métricas](#paso-9--verificar-health-checks-y-métricas)
   - [Paso 10 — Primer flujo de prueba](#paso-10--primer-flujo-de-prueba)
9. [Hot-Reload de un flujo en producción](#9-hot-reload-de-un-flujo-en-producción)
10. [Monitorización con Prometheus](#10-monitorización-con-prometheus)
11. [Despliegue local con Docker Compose](#11-despliegue-local-con-docker-compose)
12. [Variables de entorno de referencia](#12-variables-de-entorno-de-referencia)
13. [Resolución de problemas](#13-resolución-de-problemas)

---

## 1. Resumen del Proyecto

**FlowJS-Works** es una plataforma iPaaS (Integration Platform as a Service) de alto rendimiento diseñada para sustituir herramientas legacy basadas en XML (como TIBCO BusinessWorks) por una arquitectura de microservicios ligera, nativa en JSON y extensible mediante JavaScript/TypeScript.

### Características principales

| Característica | Detalle |
|---|---|
| **Lenguaje del motor** | Go 1.24 |
| **Sandbox de transformaciones** | Goja (JavaScript Engine en Go) |
| **Base de datos** | PostgreSQL 15 con JSONB |
| **Bus de mensajes** | NATS 2.9 |
| **Frontend** | React 18 + React Flow + TypeScript + Tailwind CSS |
| **Contenedores** | Docker (multi-stage builds) |
| **Orquestación** | Kubernetes (K8s) nativo |
| **Métricas** | Prometheus (formato texto, sin dependencias externas) |

---

## 2. Arquitectura General

```
┌─────────────────────────── Control Plane ──────────────────────────────┐
│                                                                         │
│   Designer UI (React Flow)  ◄──►  Engine API (Go, :9090)               │
│                                        │                                │
│                                   Config DB (Postgres - flowjs_config)  │
│                                   └─ processes  └─ secrets              │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────── Data Plane (por flujo) ─────────────────────┐
│                                                                         │
│   Trigger ──► Go Runtime Engine ──► JS Sandbox (Goja)                  │
│                      │                                                  │
│                      └──► NATS (audit.logs) ──► Audit Logger ──►       │
│                                                 Audit DB (Postgres)     │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────── Orquestador K8s ────────────────────────────┐
│                                                                         │
│   Orchestrator Service ──► K8s API Server                              │
│   (escucha NATS lifecycle events)                                       │
│   deployed/reloaded → crea/actualiza Deployment por flujo              │
│   stopped           → elimina Deployment del flujo                     │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Resumen de lo Implementado

### Fase 0 — Fundación y Contratos

Establecimiento de la base de conocimiento, contratos DSL y especificaciones API.

| Entregable | Ubicación |
|---|---|
| Base de conocimiento para agentes IA | `context/agents.md` |
| Tipos TypeScript del DSL completo | `apps/designer/src/types/dsl.ts` |
| Modelos Go del DSL | `services/engine/internal/models/process.go` |
| Especificación OpenAPI del Manager | `context/openapi.yaml` |
| Schema de base de datos (config + audit) | `init-db/` |
| Plan de ejecución por fases | `docs/execution-plan.md` |
| Análisis de riesgos técnicos | `docs/risk-analysis.md` |

---

### Fase 1 — Motor de Ejecución (Engine)

Motor Go que interpreta el DSL JSON y ejecuta flujos de integración.

**Estructura del servicio:**
```
services/engine/
├── cmd/server/main.go          # Servidor HTTP principal (:9090)
├── cmd/runner/main.go          # CLI para pruebas locales
└── internal/
    ├── activities/              # Nodos de actividad (ver catálogo completo)
    ├── engine/executor.go       # Motor de ejecución con transiciones
    ├── metrics/metrics.go       # Contadores Prometheus (sin dependencias)
    ├── middleware/security.go   # CORS, Rate-limit, SecurityHeaders, HSTS
    ├── models/                  # Modelos DSL (Process, Node, Trigger, Context)
    ├── secrets/                 # Store AES-256-GCM + resolución en runtime
    ├── store/process_store.go   # CRUD de procesos en PostgreSQL
    └── triggers/                # Gestores de trigger (cron, rest, soap, etc.)
```

**Capacidades del motor:**
- Ejecución secuencial y basada en transiciones (`success` / `error` / `condition` / `nocondition`)
- Resolución de variables JSONPath (`$.trigger.body`, `$.nodes.ID.output`)
- Política de reintentos configurable por nodo (max_attempts, interval, exponential)
- Contexto de ejecución inmutable por nodo con UUID único
- Publicación asíncrona de eventos de auditoría en NATS (sujeto `audit.logs`)
- Apagado graceful con `terminationGracePeriodSeconds: 30`

---

### Fase 2 — Módulo de Secretos

Almacén centralizado de credenciales con cifrado AES-256-GCM.

| Componente | Descripción |
|---|---|
| Tabla `secrets` en `flowjs_config` | Almacena el valor cifrado (BYTEA) + metadatos JSONB |
| `SecretStore` (Go) | CRUD: Create/Upsert, List (sin valores), Delete |
| `SecretResolver` (interfaz) | Resuelve `secret_ref` en `config` de nodo en tiempo de ejecución |
| API REST `/api/v1/secrets` | GET (lista metadatos), POST (crear/actualizar), DELETE /{id} |
| Scrubbing de auditoría | Los valores de secretos nunca aparecen en `input_data`/`output_data` |

> **Seguridad:** Si `APP_ENV != "development"` y `SECRETS_AES_KEY` tiene menos de 32 bytes, el servidor **rechaza el arranque** (fail-fast).

---

### Fase 3 — Diseñador Visual (Designer UI)

SPA React con editor visual drag-and-drop para diseño de flujos.

```
apps/designer/src/
├── App.tsx                          # Layout principal, canvas React Flow
├── components/
│   ├── ExecutionHistory.tsx         # Historial de ejecuciones con logs paso a paso
│   ├── NodeConfigPanel.tsx          # Panel de configuración por nodo
│   ├── FlowNode.tsx                 # Componente de nodo genérico
│   ├── SecretsManager.tsx           # Gestión de secretos cifrados
│   └── TriggerConfig.tsx            # Configuración de triggers
├── lib/
│   ├── api.ts                       # Cliente HTTP del Engine + Audit Logger
│   ├── serializer.ts                # Grafo React Flow ↔ DSL JSON
│   ├── errors.ts                    # Helper toErrorMessage()
│   └── classNames.ts                # Clases Tailwind reutilizables
└── types/
    ├── dsl.ts                       # Tipos del DSL completo
    ├── audit.ts                     # Tipos de Execution + ActivityLog
    ├── deployment.ts                # ProcessSummary, DeploymentStatus
    └── secrets.ts                   # SecretMeta, SecretInput
```

**Características del diseñador:**
- Canvas drag-and-drop basado en React Flow (@xyflow/react)
- Editor de scripts JavaScript con Monaco Editor (igual que VS Code)
- Serialización bidireccional grafo ↔ DSL JSON
- Ejecución en vivo de nodos individuales (live test)
- Historial de ejecuciones con inspector de payloads paso a paso
- Replay completo y parcial (desde un nodo específico)
- Gestión de secretos (crear, listar, eliminar) sin mostrar valores

---

### Fase 4 — Nodos de Transferencia de Ficheros

| Nodo | Tipo DSL | Descripción |
|---|---|---|
| SFTP | `sftp` | Get/Put de ficheros con filtro por regex, creación de carpetas |
| S3 | `s3` | Operaciones sobre buckets AWS (put/get/delete/list) vía AWS SDK v2 |
| SMB | `smb` | Cliente SMB2/3 para shares Windows (read/write/delete/list) |
| File | `file` | Sistema de ficheros local (create/read/append/delete) |

---

### Fase 5 — Triggers y Despliegues

Sistema completo de ciclo de vida de triggers y API de despliegue.

| Trigger | Tipo DSL | Descripción |
|---|---|---|
| Manual | `manual` | Iniciado desde la UI o vía `POST /api/v1/processes/{id}/replay` |
| REST | `rest` | Webhook HTTP (monta ruta en `/triggers/{path}`) |
| SOAP | `soap` | Endpoint XML/SOAP (monta ruta en `/soap/{path}`) |
| Cron | `cron` | Scheduler tipo cron (robfig/cron v3) |
| RabbitMQ | `rabbitmq` | Consumidor AMQP que alimenta ejecuciones de flujo |
| MCP | `mcp` | Listener del Model Context Protocol (IA) |

**API de ciclo de vida de flujos:**
```
POST /api/v1/processes/{id}/deploy    → Activa el trigger, status = "deployed"
POST /api/v1/processes/{id}/stop      → Desactiva el trigger, status = "stopped"
POST /api/v1/processes/{id}/reload    → Hot-reload del DSL (sin pérdida de mensajes)
POST /api/v1/processes/{id}/replay    → Re-ejecuta con nuevos datos de trigger
POST /api/v1/processes/{id}/replay-from/{nodeId} → Replay parcial desde un nodo
```

---

### Fase 6 — Observabilidad y Replay

#### Audit Logger Service (`services/audit-logger`)

Microservicio independiente que consume eventos de NATS y los persiste en PostgreSQL con procesamiento por lotes.

```
services/audit-logger/
├── cmd/main.go                   # Servidor HTTP (:8080) + suscriptor NATS
└── internal/
    ├── batcher/batcher.go        # Acumula eventos y hace flush por lote (100 eventos / 5s)
    ├── db/db.go                  # Persistencia BatchInsertLogs en PostgreSQL
    ├── metrics/metrics.go        # Contadores Prometheus del audit-logger
    ├── middleware/security.go    # Mismo stack de seguridad que el engine
    └── subscriber/subscriber.go  # Suscriptor NATS con reconexión automática
```

**Schema de auditoría (PostgreSQL):**
```sql
-- flowjs_audit
executions     → execution_id (UUID), flow_id, version, status, correlation_id,
                 start_time, end_time, trigger_type, main_error_message
activity_logs  → log_id, execution_id, node_id, node_type, status,
                 input_data (JSONB), output_data (JSONB), error_details (JSONB),
                 duration_ms, created_at
```
Índices GIN en `input_data` y `output_data` para búsquedas JSONB eficientes.

#### API de auditoría:
```
GET /executions                          → Lista ejecuciones (paginación, filtros status/search)
GET /executions/{id}/logs                → Logs paso a paso de una ejecución
GET /executions/{id}/trigger-data        → Payload del trigger original
```

---

### Fase 7 — Cloud-Native y Escala

#### 7.1 Dockerfiles Multi-Etapa

| Servicio | Imagen base builder | Imagen base runner | Puerto |
|---|---|---|---|
| `services/engine` | `golang:1.24-alpine` | `alpine:3.19` | 9090 |
| `services/audit-logger` | `golang:1.24-alpine` | `alpine:3.19` | 8080 |
| `services/orchestrator` | `golang:1.24-alpine` | `alpine:3.19` | 8081 |
| `apps/designer` | `node:22-alpine` | `nginx:1.27-alpine` | 80 |

#### 7.2 Manifiestos Kubernetes (`deploy/k8s/`)

```
deploy/k8s/
├── kustomization.yaml            # Punto de entrada (kubectl apply -k)
├── namespace.yaml                # Namespace "flowjs"
├── nats/nats.yaml                # Deployment + Service NATS 2.9
├── postgres/postgres.yaml        # Deployment + Service + PVC (5Gi) + Secret
├── engine/engine.yaml            # Deployment + Service + ConfigMap + Secret
├── audit-logger/audit-logger.yaml# Deployment + Service + ConfigMap + Secret
├── designer/designer.yaml        # Deployment + Service + ConfigMap
└── orchestrator/orchestrator.yaml# Deployment + Service + ServiceAccount + RBAC
```

Todos los Deployments de backend utilizan estrategia **RollingUpdate** (`maxSurge: 1`, `maxUnavailable: 0`).

#### 7.3 Orquestador de Flujos (`services/orchestrator`)

Microservicio Go que escucha eventos de ciclo de vida en NATS y gestiona **Deployments Kubernetes por flujo** (aislamiento total de recursos).

```
services/orchestrator/
├── cmd/server/main.go
└── internal/
    ├── api/server.go             # Suscriptor NATS + servidor HTTP (/health, /ready)
    └── controller/controller.go  # Cliente K8s API (crea/actualiza/elimina Deployments)
```

**Comportamiento:**
- `deployed` / `reloaded` → Crea o actualiza un `Deployment` dedicado al flujo (nombre: `flow-{id}`)
- `stopped` → Elimina el `Deployment` del flujo

**RBAC asignado al ServiceAccount `flowjs-orchestrator`:**
```yaml
resources: [deployments, services]
verbs:     [get, list, create, update, patch, delete]
```

#### 7.4 Health Checks y Métricas Prometheus

Todos los servicios Go exponen tres endpoints de observabilidad:

| Endpoint | Tipo | Descripción |
|---|---|---|
| `GET /health` | Liveness probe | Devuelve `200 OK` en cuanto el proceso arranca |
| `GET /ready` | Readiness probe | Devuelve `503` hasta que todos los recursos están inicializados |
| `GET /metrics` | Prometheus scrape | Métricas en formato texto, sin dependencias externas |

**Métricas del Engine:**
```
flowjs_engine_executions_total
flowjs_engine_executions_success_total
flowjs_engine_executions_error_total
flowjs_engine_http_requests_total
flowjs_engine_nats_publish_total
flowjs_engine_nats_publish_errors_total
flowjs_engine_ready
```

**Métricas del Audit Logger:**
```
flowjs_auditlogger_events_received_total
flowjs_auditlogger_events_persisted_total
flowjs_auditlogger_batches_flushed_total
flowjs_auditlogger_batch_flush_errors_total
flowjs_auditlogger_http_requests_total
flowjs_auditlogger_ready
```

#### 7.5 Hot-Reload sin pérdida de mensajes

```
POST /api/v1/processes/{id}/reload
```

El endpoint re-lee el DSL más reciente de la base de datos y llama a `TriggerManager.Deploy()`, que bajo un mutex:
1. Detiene el handler de trigger existente
2. Inicia un nuevo handler con el DSL actualizado

El `TriggerManager` garantiza que no se pierden mensajes en vuelo durante el swap. Los Deployments K8s utilizan `maxUnavailable: 0` para que siempre haya al menos una réplica activa.

---

## 4. Catálogo de Actividades

| Tipo DSL | Nombre | Descripción |
|---|---|---|
| `logger` | Logger | Log a consola con niveles configurable (legacy) |
| `log` | Log | Logger mejorado con niveles ERROR/WARNING/INFO/DEBUG |
| `http` | HTTP | Cliente HTTP completo (métodos, headers, timeout, retry, auth) |
| `code` | Code | Sandbox JavaScript con Goja (sin acceso a FS/red) |
| `sql` | SQL | Consultas SQL en PostgreSQL y MySQL |
| `mail` | Mail | Envío de correo vía SMTP |
| `rabbitmq` | RabbitMQ | Publicación de mensajes AMQP |
| `sftp` | SFTP | Transferencia de ficheros vía SFTP/SSH |
| `s3` | S3 | Operaciones en buckets AWS S3 (SDK v2) |
| `smb` | SMB | Acceso a shares Windows (SMB2/3) |
| `file` | File | Operaciones en sistema de ficheros local |
| `transform` | Transform | Conversión de formatos (JSON↔CSV, JSON↔XML) |

---

## 5. Catálogo de Triggers

| Tipo DSL | Descripción | Config relevante |
|---|---|---|
| `manual` | No inicia automáticamente; se dispara por API | — |
| `rest` | Webhook HTTP entrante | `path`, `method` |
| `soap` | Endpoint SOAP/XML entrante | `path` |
| `cron` | Scheduler tipo cron | `expression` (formato cron estándar) |
| `rabbitmq` | Consumidor de cola AMQP | `url`, `queue`, `exchange` |
| `mcp` | Listener Model Context Protocol (IA) | `transport`, `tools` |

---

## 6. API REST del Engine

**Base URL:** `http://engine:9090`

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/health` | Liveness probe |
| `GET` | `/ready` | Readiness probe |
| `GET` | `/metrics` | Métricas Prometheus |
| `POST` | `/v1/flow` | Ejecutar un flujo DSL completo |
| `POST` | `/v1/test` | Probar un nodo individual en vivo |
| `GET` | `/api/v1/secrets` | Listar secretos (sin valores) |
| `POST` | `/api/v1/secrets` | Crear o actualizar secreto |
| `DELETE` | `/api/v1/secrets/{id}` | Eliminar secreto |
| `GET` | `/api/v1/processes` | Listar todos los flujos |
| `POST` | `/api/v1/processes` | Crear o actualizar flujo |
| `GET` | `/api/v1/processes/{id}` | Obtener DSL completo de un flujo |
| `DELETE` | `/api/v1/processes/{id}` | Eliminar flujo |
| `POST` | `/api/v1/processes/{id}/deploy` | Activar trigger del flujo |
| `POST` | `/api/v1/processes/{id}/stop` | Desactivar trigger del flujo |
| `POST` | `/api/v1/processes/{id}/reload` | Hot-reload del DSL |
| `POST` | `/api/v1/processes/{id}/replay` | Re-ejecutar flujo con nuevos datos |
| `POST` | `/api/v1/processes/{id}/replay-from/{nodeId}` | Replay parcial |
| `POST` | `/triggers/{path}` | Entrada de trigger REST desplegado |
| `POST` | `/soap/{path}` | Entrada de trigger SOAP desplegado |

---

## 7. Endpoints del Audit Logger

**Base URL:** `http://audit-logger:8080`

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/health` | Liveness probe (con ping a PostgreSQL) |
| `GET` | `/ready` | Readiness probe |
| `GET` | `/metrics` | Métricas Prometheus |
| `GET` | `/executions` | Lista ejecuciones (`?status=`, `?search=`, `?limit=`, `?offset=`) |
| `GET` | `/executions/{id}/logs` | Logs de actividad de una ejecución |
| `GET` | `/executions/{id}/trigger-data` | Payload del trigger original |

---

## 8. Guía de Despliegue en Kubernetes

### 8.1 Prerrequisitos

| Herramienta | Versión mínima | Descripción |
|---|---|---|
| `kubectl` | 1.28+ | CLI de Kubernetes |
| `kustomize` | 5.0+ | Incluido en `kubectl apply -k` |
| Docker / containerd | 24+ | Para construir imágenes |
| Cluster Kubernetes | 1.28+ | Local (minikube, kind) o cloud (EKS, GKE, AKS) |
| Registro de imágenes | — | Docker Hub, GHCR, ECR, GCR, etc. |
| PostgreSQL | 15+ | En cluster (PVC) o gestionado externamente |

> **Nota para entornos locales:** Se puede usar [minikube](https://minikube.sigs.k8s.io/) o [kind](https://kind.sigs.k8s.io/) para probar el despliegue completo sin coste de nube.

---

### Paso 1 — Clonar el repositorio

```bash
git clone https://github.com/jufeza-boop/flowjs-works.git
cd flowjs-works
```

---

### Paso 2 — Construir las imágenes Docker

Construir todas las imágenes desde la raíz del repositorio. Sustituir `<tu-registry>` por tu registro de contenedores (ej. `docker.io/miusuario`, `ghcr.io/miorg`, etc.).

```bash
export REGISTRY=<tu-registry>
export TAG=1.0.0

# Engine
docker build -t ${REGISTRY}/flowjs-engine:${TAG} \
  -f services/engine/Dockerfile \
  services/engine/

# Audit Logger
docker build -t ${REGISTRY}/flowjs-audit-logger:${TAG} \
  -f services/audit-logger/Dockerfile \
  services/audit-logger/

# Orchestrator
docker build -t ${REGISTRY}/flowjs-orchestrator:${TAG} \
  -f services/orchestrator/Dockerfile \
  services/orchestrator/

# Designer (requiere las URLs de los backends como build args)
docker build -t ${REGISTRY}/flowjs-designer:${TAG} \
  --build-arg VITE_ENGINE_API_URL=http://<ip-o-dominio-engine>:9090 \
  --build-arg VITE_AUDIT_API_URL=http://<ip-o-dominio-audit>:8080 \
  -f apps/designer/Dockerfile \
  apps/designer/
```

> **⚠️ Importante:** Las variables `VITE_*` del designer se compilan dentro del bundle JavaScript en tiempo de build. Deben apuntar a las URLs públicas o internas accesibles desde el navegador del usuario.

---

### Paso 3 — Publicar las imágenes en un registro

```bash
docker push ${REGISTRY}/flowjs-engine:${TAG}
docker push ${REGISTRY}/flowjs-audit-logger:${TAG}
docker push ${REGISTRY}/flowjs-orchestrator:${TAG}
docker push ${REGISTRY}/flowjs-designer:${TAG}
```

---

### Paso 4 — Configurar secretos de producción

Los manifiestos en `deploy/k8s/` contienen valores de placeholder. **Antes de aplicarlos en producción**, actualiza los siguientes valores sensibles:

#### 4.1 Clave AES del Engine (`deploy/k8s/engine/engine.yaml`)

```bash
# Generar una clave aleatoria de 32 bytes
AES_KEY=$(openssl rand -hex 32)
echo "AES_KEY: ${AES_KEY}"
```

Editar `deploy/k8s/engine/engine.yaml` y reemplazar:
```yaml
stringData:
  SECRETS_AES_KEY: "CHANGE_ME_use_32_bytes_random_key"
# → sustituir por el valor generado arriba
```

> **Alternativa recomendada en producción:** Usar [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) o un operador de gestión de secretos externos (AWS Secrets Manager, HashiCorp Vault).

#### 4.2 Contraseñas de PostgreSQL (`deploy/k8s/postgres/postgres.yaml`)

Los valores actuales son los defaults de desarrollo en base64:
- `YWRtaW4=` → `admin`
- `Zmxvd2pzX3Bhc3M=` → `flowjs_pass`

Para producción, generar nuevas credenciales:
```bash
echo -n "mi_usuario_seguro" | base64
echo -n "mi_contraseña_segura" | base64
```
Actualizar `deploy/k8s/postgres/postgres.yaml`:
```yaml
data:
  POSTGRES_USER: <base64 del usuario>
  POSTGRES_PASSWORD: <base64 de la contraseña>
```

#### 4.3 DSN del Audit Logger (`deploy/k8s/audit-logger/audit-logger.yaml`)

Actualizar el `stringData.POSTGRES_DSN` con las credenciales correctas:
```yaml
stringData:
  POSTGRES_DSN: "host=postgres port=5432 user=MI_USUARIO password=MI_PASSWORD dbname=flowjs_audit sslmode=require"
```

#### 4.4 Actualizar las referencias de imagen

Editar cada `image:` en los manifiestos para apuntar al registro propio:
```bash
# Usando sed para actualizar todas las referencias
sed -i "s|flowjs-engine:latest|${REGISTRY}/flowjs-engine:${TAG}|g" deploy/k8s/engine/engine.yaml
sed -i "s|flowjs-audit-logger:latest|${REGISTRY}/flowjs-audit-logger:${TAG}|g" deploy/k8s/audit-logger/audit-logger.yaml
sed -i "s|flowjs-orchestrator:latest|${REGISTRY}/flowjs-orchestrator:${TAG}|g" deploy/k8s/orchestrator/orchestrator.yaml
sed -i "s|flowjs-designer:latest|${REGISTRY}/flowjs-designer:${TAG}|g" deploy/k8s/designer/designer.yaml
```

---

### Paso 5 — Inicializar las bases de datos

Los scripts de inicialización están en `init-db/`. Si usas el PostgreSQL del clúster (el del manifiesto K8s), espera a que esté listo y ejecuta:

```bash
# Obtener el nombre del pod de PostgreSQL
POSTGRES_POD=$(kubectl -n flowjs get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Copiar los scripts al pod
kubectl -n flowjs cp init-db/01-init.sql       ${POSTGRES_POD}:/tmp/01-init.sql
kubectl -n flowjs cp init-db/02-audit-schema.sql  ${POSTGRES_POD}:/tmp/02-audit-schema.sql
kubectl -n flowjs cp init-db/03-config-schema.sql ${POSTGRES_POD}:/tmp/03-config-schema.sql

# Ejecutar los scripts en orden
kubectl -n flowjs exec ${POSTGRES_POD} -- \
  psql -U admin -f /tmp/01-init.sql
kubectl -n flowjs exec ${POSTGRES_POD} -- \
  psql -U admin -f /tmp/02-audit-schema.sql
kubectl -n flowjs exec ${POSTGRES_POD} -- \
  psql -U admin -f /tmp/03-config-schema.sql
```

> **Nota:** El script `01-init.sql` crea las bases de datos `flowjs_audit` y `flowjs_config`. Los scripts `02` y `03` crean las tablas y los índices GIN.

---

### Paso 6 — Aplicar los manifiestos Kubernetes

Se utiliza **Kustomize** para aplicar todos los recursos de una sola vez:

```bash
# Aplicar todos los recursos del namespace flowjs
kubectl apply -k deploy/k8s/

# Verificar que el namespace se creó correctamente
kubectl get namespace flowjs
```

Esto creará en orden:
1. Namespace `flowjs`
2. NATS (Deployment + Service)
3. PostgreSQL (Secret + PVC + Deployment + Service)
4. Audit Logger (ConfigMap + Secret + Deployment + Service)
5. Engine (ConfigMap + Secret + Deployment + Service)
6. Designer (ConfigMap + Deployment + Service)
7. Orchestrator (ServiceAccount + Role + RoleBinding + ConfigMap + Deployment + Service)

---

### Paso 7 — Verificar el estado del despliegue

```bash
# Ver todos los pods del namespace flowjs
kubectl -n flowjs get pods -w

# El resultado esperado (todos en Running):
# NAME                           READY   STATUS    RESTARTS   AGE
# nats-xxx                       1/1     Running   0          2m
# postgres-xxx                   1/1     Running   0          2m
# audit-logger-xxx               1/1     Running   0          1m
# engine-xxx                     1/1     Running   0          1m
# designer-xxx                   1/1     Running   0          1m
# orchestrator-xxx               1/1     Running   0          1m

# Ver servicios
kubectl -n flowjs get services

# Ver eventos recientes (útil para depuración)
kubectl -n flowjs get events --sort-by='.lastTimestamp'

# Ver logs de un servicio específico
kubectl -n flowjs logs -l app=engine --tail=50
kubectl -n flowjs logs -l app=audit-logger --tail=50
```

---

### Paso 8 — Exponer la UI al exterior

Por defecto, el servicio `designer` es de tipo `ClusterIP`. Para acceder desde el exterior:

#### Opción A: Port-forward para pruebas locales

```bash
# Acceder al Designer UI en http://localhost:8080
kubectl -n flowjs port-forward svc/designer 8080:80 &

# Acceder al Engine API en http://localhost:9090
kubectl -n flowjs port-forward svc/engine 9090:9090 &

# Acceder al Audit Logger API en http://localhost:8081
kubectl -n flowjs port-forward svc/audit-logger 8081:8080 &
```

#### Opción B: Ingress (recomendado para producción)

Crear un recurso `Ingress` (requiere un Ingress Controller como nginx-ingress o Traefik):

```yaml
# deploy/k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: flowjs-ingress
  namespace: flowjs
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
    - host: flowjs.mi-empresa.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: designer
                port:
                  number: 80
    - host: api.flowjs.mi-empresa.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: engine
                port:
                  number: 9090
```

```bash
kubectl apply -f deploy/k8s/ingress.yaml
```

#### Opción C: LoadBalancer (para cloud providers)

```bash
kubectl -n flowjs patch svc designer -p '{"spec":{"type":"LoadBalancer"}}'
kubectl -n flowjs get svc designer  # Obtener la EXTERNAL-IP asignada
```

---

### Paso 9 — Verificar health checks y métricas

```bash
# Verificar liveness probe del engine
kubectl -n flowjs exec -it $(kubectl -n flowjs get pod -l app=engine -o jsonpath='{.items[0].metadata.name}') \
  -- wget -qO- http://localhost:9090/health

# Verificar readiness probe
kubectl -n flowjs exec -it $(kubectl -n flowjs get pod -l app=engine -o jsonpath='{.items[0].metadata.name}') \
  -- wget -qO- http://localhost:9090/ready

# Ver métricas Prometheus del engine
kubectl -n flowjs exec -it $(kubectl -n flowjs get pod -l app=engine -o jsonpath='{.items[0].metadata.name}') \
  -- wget -qO- http://localhost:9090/metrics

# Verificar el audit-logger
kubectl -n flowjs exec -it $(kubectl -n flowjs get pod -l app=audit-logger -o jsonpath='{.items[0].metadata.name}') \
  -- wget -qO- http://localhost:8080/health
```

Respuesta esperada del endpoint `/health`:
```json
{"status": "ok", "service": "engine"}
```

Respuesta esperada del endpoint `/ready`:
```json
{"status": "ready", "service": "engine"}
```

---

### Paso 10 — Primer flujo de prueba

Una vez todo esté corriendo, crear y ejecutar un flujo simple de prueba:

```bash
# Con port-forward activo en :9090

# 1. Crear un flujo de prueba
curl -X POST http://localhost:9090/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{
    "definition": {
      "id": "flujo-hola-mundo",
      "version": "1.0.0",
      "name": "Hola Mundo"
    },
    "trigger": {
      "id": "trg_manual",
      "type": "manual"
    },
    "nodes": [
      {
        "id": "log_saludo",
        "type": "log",
        "input_mapping": {
          "message": "$.trigger.body.nombre"
        },
        "config": {
          "level": "INFO"
        }
      }
    ]
  }'

# 2. Ejecutar el flujo con datos de trigger
curl -X POST http://localhost:9090/api/v1/processes/flujo-hola-mundo/replay \
  -H "Content-Type: application/json" \
  -d '{"trigger_data": {"nombre": "FlowJS"}}'

# 3. Ver el historial de ejecuciones (requiere port-forward en :8081)
curl http://localhost:8081/executions
```

---

## 9. Hot-Reload de un flujo en producción

Para actualizar el DSL de un flujo desplegado **sin interrupciones**:

```bash
# 1. Actualizar el DSL en la base de datos
curl -X POST http://localhost:9090/api/v1/processes \
  -H "Content-Type: application/json" \
  -d '{ ... nuevo DSL ... }'

# 2. Disparar el hot-reload (no detiene el trigger existente hasta que el nuevo esté listo)
curl -X POST http://localhost:9090/api/v1/processes/mi-flujo/reload

# Respuesta esperada:
# {"process_id":"mi-flujo","status":"deployed","message":"rest trigger reloaded with latest DSL"}
```

El `TriggerManager` utiliza un mutex para garantizar que la transición viejo→nuevo handler es atómica y no se pierden mensajes en vuelo.

En Kubernetes, el rolling update (`maxSurge: 1`, `maxUnavailable: 0`) asegura que siempre hay al menos una réplica activa durante la actualización de la imagen.

---

## 10. Monitorización con Prometheus

Los pods de Engine y Audit Logger tienen las siguientes anotaciones que permiten a Prometheus descubrirlos automáticamente:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/path: "/metrics"
  prometheus.io/port: "9090"   # o 8080 para audit-logger
```

Si tienes Prometheus instalado en el clúster (ej. via kube-prometheus-stack), emezará a recopilar métricas automáticamente.

**Consultas PromQL de ejemplo:**

```promql
# Tasa de ejecuciones por segundo
rate(flowjs_engine_executions_total[5m])

# Tasa de error en los últimos 5 minutos
rate(flowjs_engine_executions_error_total[5m])

# Ratio de éxito (%)
rate(flowjs_engine_executions_success_total[5m])
  / rate(flowjs_engine_executions_total[5m]) * 100

# Eventos de auditoría recibidos vs persistidos
rate(flowjs_auditlogger_events_received_total[5m])
rate(flowjs_auditlogger_events_persisted_total[5m])

# Errores de publicación NATS
increase(flowjs_engine_nats_publish_errors_total[1h])
```

---

## 11. Despliegue local con Docker Compose

Para desarrollo local sin necesidad de un clúster Kubernetes:

```bash
# 1. Copiar el fichero de entorno
cp .env.example .env
# Editar .env y ajustar SECRETS_AES_KEY (mínimo 32 caracteres)

# 2. Levantar todos los servicios
docker compose up -d

# 3. Verificar que todos los contenedores están corriendo
docker compose ps

# Servicios disponibles:
# flowjs-db          → PostgreSQL en localhost:5432
# flowjs-bus         → NATS en localhost:4222 (monitor: 8222)
# flowjs-engine      → Engine API en localhost:9090
# flowjs-audit-logger→ Audit Logger en localhost:8080
# flowjs-designer    → Designer UI en localhost:5173
# flowjs-orchestrator→ Orchestrator en localhost:8081
# flowjs-pgadmin     → pgAdmin en localhost:5050

# 4. Abrir el Designer
open http://localhost:5173

# 5. Ver logs en tiempo real
docker compose logs -f engine
docker compose logs -f audit-logger
```

---

## 12. Variables de entorno de referencia

### Engine (`services/engine`)

| Variable | Requerida | Descripción | Default dev |
|---|---|---|---|
| `APP_ENV` | No | Modo de ejecución (`development` / `production`) | `development` |
| `NATS_URL` | No | URL del servidor NATS | `nats://localhost:4222` |
| `HTTP_ADDR` | No | Dirección de escucha del servidor HTTP | `:9090` |
| `DATABASE_URL` | No* | DSN de PostgreSQL para config/secrets | — |
| `SECRETS_AES_KEY` | **Sí (prod)** | Clave AES-256 (≥32 bytes). Fatal si falta en producción | dev-key hardcodeado |
| `ALLOWED_ORIGINS` | **Sí (prod)** | Orígenes CORS permitidos (comma-separated) | `http://localhost:5173` |
| `REQUEST_TIMEOUT` | No | Timeout máximo de petición HTTP (formato Go) | `60s` |

*\*Sin `DATABASE_URL`, los endpoints de secrets y processes devuelven 503.*

### Audit Logger (`services/audit-logger`)

| Variable | Requerida | Descripción | Default dev |
|---|---|---|---|
| `APP_ENV` | No | Modo de ejecución | `development` |
| `NATS_URL` | **Sí** | URL del servidor NATS | `nats://localhost:4222` |
| `POSTGRES_DSN` | **Sí** | DSN de PostgreSQL del audit DB | — |
| `HTTP_ADDR` | No | Dirección de escucha | `:8080` |
| `ALLOWED_ORIGINS` | **Sí (prod)** | Orígenes CORS permitidos | `http://localhost:5173` |

### Orchestrator (`services/orchestrator`)

| Variable | Requerida | Descripción | Default |
|---|---|---|---|
| `NATS_URL` | **Sí** | URL del servidor NATS | `nats://localhost:4222` |
| `HTTP_ADDR` | No | Dirección de escucha | `:8081` |
| `K8S_NAMESPACE` | No | Namespace K8s donde crear los Deployments | `flowjs` |
| `ENGINE_IMAGE` | No | Imagen Docker del engine para pods por flujo | `flowjs-engine:latest` |
| `DATABASE_URL` | No | DSN inyectado en los pods por flujo | — |
| `SECRETS_AES_KEY` | No | AES key inyectada en los pods por flujo | — |
| `KUBECONFIG_TOKEN` | No | Token K8s para desarrollo fuera del clúster | — |
| `KUBECONFIG_API_SERVER` | No | URL del API server K8s para desarrollo | `https://kubernetes.default.svc` |

### Designer (`apps/designer`)

| Variable | Descripción | Default |
|---|---|---|
| `VITE_ENGINE_API_URL` | URL del Engine API (baked en build-time) | `http://localhost:9090` |
| `VITE_AUDIT_API_URL` | URL del Audit Logger API (baked en build-time) | `http://localhost:8080` |

---

## 13. Resolución de problemas

### Los pods no arrancan (CrashLoopBackOff)

```bash
# Ver los logs del pod con error
kubectl -n flowjs logs <nombre-del-pod> --previous

# Ver los eventos del pod
kubectl -n flowjs describe pod <nombre-del-pod>
```

**Causas comunes:**
- `SECRETS_AES_KEY` con menos de 32 caracteres en modo producción
- `POSTGRES_DSN` incorrecto (contraseña, hostname o base de datos erróneos)
- PostgreSQL no ha terminado de inicializarse antes que los servicios dependientes
- Imágenes Docker no encontradas en el registro (ImagePullBackOff)

### El Engine devuelve 503 en `/api/v1/processes`

La variable `DATABASE_URL` no está configurada. El engine funciona sin base de datos, pero los endpoints de procesos y secretos no están disponibles.

### La UI del Designer no puede conectar al Engine

Verificar que `VITE_ENGINE_API_URL` y `VITE_AUDIT_API_URL` se compilaron con las URLs correctas:
```bash
# Dentro del contenedor del designer, ver la config compilada
kubectl -n flowjs exec -it $(kubectl -n flowjs get pod -l app=designer -o jsonpath='{.items[0].metadata.name}') \
  -- grep -r "VITE" /usr/share/nginx/html/assets/*.js | head -5
```

### NATS no acepta conexiones

```bash
# Verificar que NATS está corriendo y accesible
kubectl -n flowjs exec -it $(kubectl -n flowjs get pod -l app=engine -o jsonpath='{.items[0].metadata.name}') \
  -- wget -qO- http://nats:8222/healthz
```

### El orquestador no crea Deployments por flujo

Verificar los permisos RBAC:
```bash
# Comprobar que el ServiceAccount tiene los permisos correctos
kubectl -n flowjs auth can-i create deployments \
  --as=system:serviceaccount:flowjs:flowjs-orchestrator

# Ver los logs del orquestador
kubectl -n flowjs logs -l app=orchestrator --tail=50
```

### Rollback a una versión anterior

```bash
# Ver el historial de cambios de un Deployment
kubectl -n flowjs rollout history deployment/engine

# Hacer rollback a la revisión anterior
kubectl -n flowjs rollout undo deployment/engine

# Hacer rollback a una revisión específica
kubectl -n flowjs rollout undo deployment/engine --to-revision=2
```

---

*Documento generado automáticamente a partir del código fuente de flowjs-works.*  
*Para contribuir o reportar errores, abrir un issue en el repositorio.*
