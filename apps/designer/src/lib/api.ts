import type { Execution, ActivityLog } from '../types/audit'
import type { InputMapping, FlowDSL } from '../types/dsl'
import type { SecretMeta, SecretInput } from '../types/secrets'

/** Base URL for the audit-logger HTTP API */
const AUDIT_API_BASE = import.meta.env.VITE_AUDIT_API_URL ?? 'http://localhost:8080'

/** Base URL for the engine HTTP API */
const ENGINE_API_BASE = import.meta.env.VITE_ENGINE_API_URL ?? 'http://localhost:9090'

/** Payload sent to the engine live-test endpoint */
export interface LiveTestRequest {
  input_mapping: InputMapping
  script?: string
  input_payload: Record<string, unknown>
  /** The DSL node type to execute (e.g. 'log', 'http', 'sql'). Defaults to 'logger' when absent. */
  node_type?: string
  /** Node config forwarded verbatim to the activity (e.g. {level, message} for log nodes). */
  config?: Record<string, unknown>
}

/** Response from the engine live-test endpoint */
export interface LiveTestResponse {
  output: Record<string, unknown>
  error?: string
  duration_ms?: number
}

/**
 * Sends a live-test request to the engine service and returns the transformation result.
 * Endpoint: POST /v1/test
 */
export async function liveTest(payload: LiveTestRequest): Promise<LiveTestResponse> {
  const res = await fetch(`${ENGINE_API_BASE}/v1/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`Live test failed (${res.status}): ${body}`)
  }
  return res.json() as Promise<LiveTestResponse>
}

/** Request payload for running a full DSL flow */
export interface RunFlowRequest {
  dsl: FlowDSL
  trigger_data?: Record<string, unknown>
}

/** Node execution result within a flow run response */
export interface NodeResult {
  output?: Record<string, unknown>
  status?: string
}

/** Flat node result item in the node_results array */
export interface NodeResultItem {
  node_id: string
  status: string
  output?: Record<string, unknown>
}

/** Response from the engine run-flow endpoint */
export interface RunFlowResponse {
  execution_id: string
  nodes: Record<string, NodeResult>
  /** Flat array of per-node execution results (used by DebugPanel) */
  node_results?: NodeResultItem[]
  error?: string
}

/**
 * Sends a complete DSL flow to the engine for execution.
 * Endpoint: POST /v1/flow
 */
export async function runFlow(payload: RunFlowRequest): Promise<RunFlowResponse> {
  const res = await fetch(`${ENGINE_API_BASE}/v1/flow`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  const data = await res.json() as RunFlowResponse
  if (!res.ok) {
    throw new Error(`Run flow failed (${res.status}): ${data.error ?? res.statusText}`)
  }
  return data
}

/** Fetch all executions ordered by start_time DESC */
export async function fetchExecutions(): Promise<Execution[]> {
  const res = await fetch(`${AUDIT_API_BASE}/executions`)
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`Failed to fetch executions (${res.status}): ${body}`)
  }
  return res.json() as Promise<Execution[]>
}

/** Fetch activity logs for a given execution_id */
export async function fetchActivityLogs(executionId: string): Promise<ActivityLog[]> {
  const res = await fetch(`${AUDIT_API_BASE}/executions/${encodeURIComponent(executionId)}/logs`)
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`Failed to fetch activity logs (${res.status}): ${body}`)
  }
  return res.json() as Promise<ActivityLog[]>
}

// ── Secrets API ──────────────────────────────────────────────────────────────

/** Fetch metadata for all secrets (values are never returned) */
export async function listSecrets(): Promise<SecretMeta[]> {
  const res = await fetch(`${ENGINE_API_BASE}/api/v1/secrets`)
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`Failed to list secrets (${res.status}): ${body}`)
  }
  return res.json() as Promise<SecretMeta[]>
}

/** Create or update a secret */
export async function createSecret(input: SecretInput): Promise<void> {
  const res = await fetch(`${ENGINE_API_BASE}/api/v1/secrets`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`Failed to save secret (${res.status}): ${body}`)
  }
}

/** Delete a secret by id */
export async function deleteSecret(secretId: string): Promise<void> {
  const res = await fetch(`${ENGINE_API_BASE}/api/v1/secrets/${encodeURIComponent(secretId)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`Failed to delete secret (${res.status}): ${body}`)
  }
}
