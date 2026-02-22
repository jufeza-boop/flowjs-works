import type { Execution, ActivityLog } from '../types/audit'
import type { InputMapping, FlowDSL } from '../types/dsl'

/** Base URL for the audit-logger HTTP API */
const AUDIT_API_BASE = import.meta.env.VITE_AUDIT_API_URL ?? 'http://localhost:8080'

/** Base URL for the engine HTTP API */
const ENGINE_API_BASE = import.meta.env.VITE_ENGINE_API_URL ?? 'http://localhost:9090'

/** Payload sent to the engine live-test endpoint */
export interface LiveTestRequest {
  input_mapping: InputMapping
  script?: string
  input_payload: Record<string, unknown>
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

/** Response from the engine run-flow endpoint */
export interface RunFlowResponse {
  execution_id: string
  nodes: Record<string, NodeResult>
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
