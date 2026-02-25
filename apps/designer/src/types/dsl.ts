// =============================================================================
// flowjs-works — Core DSL Type Definitions
// =============================================================================
// This file is the canonical TypeScript contract for the JSON DSL.
// Go mirror: services/engine/internal/models/process.go
// =============================================================================

// ── Flow Settings & Definition ──────────────────────────────────────────────

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

// ── Trigger Types ───────────────────────────────────────────────────────────

/** All supported trigger types */
export type TriggerType = 'cron' | 'rest' | 'soap' | 'rabbitmq' | 'mcp' | 'manual'

/** Cron trigger configuration */
export interface CronTriggerConfig {
  expression: string
}

/** REST trigger configuration */
export interface RestTriggerConfig {
  path: string
  method: string
  schema_validation?: string
}

/** SOAP trigger configuration */
export interface SoapTriggerConfig {
  path: string
  wsdl?: string
}

/** RabbitMQ trigger configuration */
export interface RabbitMQTriggerConfig {
  url_amqp: string
  queue: string
  vhost?: string
}

/** MCP (Model Context Protocol) trigger configuration */
export interface McpTriggerConfig {
  version: string
  capabilities?: Record<string, unknown>
}

/** Manual trigger has no required config */
export type ManualTriggerConfig = Record<string, never>

/** Union of all trigger configs, keyed by trigger type */
export type TriggerConfigMap = {
  cron: CronTriggerConfig
  rest: RestTriggerConfig
  soap: SoapTriggerConfig
  rabbitmq: RabbitMQTriggerConfig
  mcp: McpTriggerConfig
  manual: ManualTriggerConfig
}

/** Trigger node — config shape depends on type */
export interface FlowTrigger<T extends TriggerType = TriggerType> {
  id: string
  type: T
  config: TriggerConfigMap[T]
}

// ── Retry Policy ────────────────────────────────────────────────────────────

/** Retry policy for nodes */
export interface RetryPolicy {
  max_attempts: number
  interval: string
  type: 'fixed' | 'exponential'
}

// ── Input Mapping ───────────────────────────────────────────────────────────

/** Input mapping values are JSONPath expressions (e.g. $.trigger.body) */
export type InputMapping = Record<string, string>

// ── Node Types ──────────────────────────────────────────────────────────────

/** All supported node types */
export type NodeType =
  | 'http'
  | 'sftp'
  | 's3'
  | 'smb'
  | 'mail'
  | 'rabbitmq'
  | 'sql'
  | 'code'
  | 'log'
  | 'transform'
  | 'file'

// ── Node Config Interfaces ──────────────────────────────────────────────────

/** HTTP node configuration */
export interface HttpNodeConfig {
  url: string
  method: string
  headers?: Record<string, string>
  data?: unknown
  auth?: string
  timeout?: number
}

/** SFTP / S3 / SMB shared file-transfer configuration */
export interface FileTransferNodeConfig {
  server: string
  port?: number
  timeout?: number
  auth?: string
  folder: string
  method: 'get' | 'put'
  /** GET-specific: regex filter for file selection */
  regex_filter?: string
  /** GET-specific: max depth date filter */
  max_depth_date?: string
  /** PUT-specific: overwrite existing files */
  overwrite?: boolean
  /** PUT-specific: create target folder if missing */
  create_folder?: boolean
}

/** S3-specific configuration (extends file transfer) */
export interface S3NodeConfig {
  bucket: string
  region: string
  auth?: string
  folder: string
  method: 'get' | 'put'
  regex_filter?: string
  overwrite?: boolean
  create_folder?: boolean
}

/** Mail node configuration — action determines sub-fields */
export interface MailNodeConfig {
  host: string
  port: number
  security?: 'none' | 'ssl' | 'tls'
  auth?: string
  action: 'send' | 'receive'
  /** Send-specific fields */
  from?: string
  to?: string[]
  cc?: string[]
  bcc?: string[]
  subject?: string
  priority?: 'low' | 'normal' | 'high'
  body?: string
  body_type?: 'text' | 'html'
  attachments?: string[]
  /** Receive-specific fields */
  filter_subject?: string
  filter_content?: string
  filter_status?: 'read' | 'unread'
  filter_has_attachment?: boolean
  max_messages?: number
}

/** RabbitMQ producer node configuration */
export interface RabbitMQNodeConfig {
  url_amqp: string
  vhost?: string
  exchange: string
  routing_key: string
  payload?: unknown
  properties?: {
    delivery_mode?: number
    headers?: Record<string, string>
  }
}

/** SQL node configuration */
export interface SqlNodeConfig {
  engine: 'postgres' | 'mysql' | 'oracle'
  host: string
  port: number
  database: string
  schema?: string
  credentials?: string
  query: string
  params?: unknown[]
  timeout?: number
  autocommit?: boolean
  ssl_mode?: string
}

/** Code (JS script) node configuration */
export interface CodeNodeConfig {
  script: string
}

/** Log node configuration */
export interface LogNodeConfig {
  level: 'ERROR' | 'WARNING' | 'INFO' | 'DEBUG'
  message: string
}

/** Transform node configuration */
export interface TransformNodeConfig {
  transform_type: 'json2csv' | 'xml2json' | 'json2xml'
  data?: unknown
  spec?: unknown
}

/** Local file operations node configuration */
export interface FileNodeConfig {
  operation: 'create' | 'delete' | 'read'
  path: string
  content?: string
  mode?: 'overwrite' | 'append'
}

/** Union of all node config types */
export type NodeConfigMap = {
  http: HttpNodeConfig
  sftp: FileTransferNodeConfig
  s3: S3NodeConfig
  smb: FileTransferNodeConfig
  mail: MailNodeConfig
  rabbitmq: RabbitMQNodeConfig
  sql: SqlNodeConfig
  code: CodeNodeConfig
  log: LogNodeConfig
  transform: TransformNodeConfig
  file: FileNodeConfig
}

// ── Flow Node ───────────────────────────────────────────────────────────────

/** A process node in the flow */
export interface FlowNode<T extends NodeType = NodeType> {
  id: string
  type: T
  description?: string
  input_mapping?: InputMapping
  config?: NodeConfigMap[T]
  /** Reference to a secret in the secrets store */
  secret_ref?: string
  retry_policy?: RetryPolicy
  next?: string[]
}

// ── Transitions ─────────────────────────────────────────────────────────────

/** Transition types between nodes */
export type TransitionType = 'success' | 'error' | 'condition' | 'nocondition'

/** A transition between nodes */
export interface FlowTransition {
  from: string
  to: string
  type: TransitionType
  /** JSONPath expression — required when type is 'condition' */
  condition?: string
}

// ── Complete Flow DSL ───────────────────────────────────────────────────────

/** Complete flow DSL document */
export interface FlowDSL {
  definition: FlowDefinition
  trigger: FlowTrigger
  nodes: FlowNode[]
  transitions: FlowTransition[]
}
