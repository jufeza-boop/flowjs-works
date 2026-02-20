/** Execution summary row from the audit database */
export interface Execution {
  execution_id: string
  flow_id: string
  version: string
  status: string
  correlation_id: string
  start_time: string
  trigger_type: string
  main_error_message: string
}

/** Activity log entry from the audit database */
export interface ActivityLog {
  log_id: number
  node_id: string
  node_type: string
  status: string
  input_data: Record<string, unknown> | null
  output_data: Record<string, unknown> | null
  error_details: Record<string, unknown> | null
  duration_ms: number
  created_at: string
}
