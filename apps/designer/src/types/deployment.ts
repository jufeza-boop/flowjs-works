// =============================================================================
// flowjs-works — Deployment Module Types
// =============================================================================
// These types mirror the Go process store in
// services/engine/internal/store/process_store.go
// =============================================================================

/** Process deployment status */
export type ProcessStatus = 'draft' | 'deployed' | 'stopped'

/** Lightweight summary returned by GET /api/v1/processes */
export interface ProcessSummary {
  id: string
  version: string
  name: string
  status: ProcessStatus
  /** DSL trigger type, e.g. "rest" | "soap" | "cron" | "rabbitmq" | "mcp" | "manual" */
  trigger_type: string
  updated_at: string
}

/** Response from POST /api/v1/processes/{id}/deploy and /stop */
export interface DeploymentStatus {
  process_id: string
  status: ProcessStatus | 'error'
  message?: string
}
