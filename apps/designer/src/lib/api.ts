import type { Execution, ActivityLog } from '../types/audit'

/** Base URL for the audit-logger HTTP API */
const AUDIT_API_BASE = import.meta.env.VITE_AUDIT_API_URL ?? 'http://localhost:8080'

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
