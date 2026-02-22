/** Settings for a flow definition */
export interface FlowSettings {
  persistence: 'full' | 'minimal' | 'none'
  timeout: number
  error_strategy: 'stop_and_rollback' | 'continue' | 'retry'
}

/** Top-level definition metadata */
export interface FlowDefinition {
  id: string
  version: string
  name: string
  description: string
  settings: FlowSettings
}

/** Trigger configuration */
export interface TriggerConfig {
  path?: string
  method?: string
  schema_validation?: string
  [key: string]: string | undefined
}

/** Trigger node */
export interface FlowTrigger {
  id: string
  type: 'http_webhook' | 'schedule' | 'manual'
  config: TriggerConfig
}

/** Retry policy for nodes */
export interface RetryPolicy {
  max_attempts: number
  interval: string
  type: 'fixed' | 'exponential'
}

/** Input mapping (JSONPath references) */
export type InputMapping = Record<string, string>

/** Configuration for an HTTP Request node */
export interface HttpRequestConfig {
  url: string
  method: string
  timeout: number
  headers: Record<string, string>
}

/** A process node */
export interface FlowNode {
  id: string
  type: 'script_ts' | 'http_request' | 'sql_insert' | 'trigger'
  description?: string
  input_mapping?: InputMapping
  script?: string
  config?: HttpRequestConfig | Record<string, string>
  retry_policy?: RetryPolicy
  next?: string[]
}

/** A transition between nodes */
export interface FlowTransition {
  from: string
  to: string
  condition?: string
}

/** Complete flow DSL */
export interface FlowDSL {
  definition: FlowDefinition
  trigger: FlowTrigger
  nodes: FlowNode[]
  transitions: FlowTransition[]
}
