// =============================================================================
// flowjs-works — Secrets Module Types
// =============================================================================
// These types mirror the Go secrets package in
// services/engine/internal/secrets/store.go
// =============================================================================

/** Supported secret credential categories */
export type SecretType =
  | 'basic_auth'
  | 'token'
  | 'certificate'
  | 'connection_string'
  | 'aws_credentials'
  | 'ssh_key'
  | 'amqp_url'

/** Non-sensitive secret metadata returned by GET /api/v1/secrets */
export interface SecretMeta {
  id: string
  name: string
  type: SecretType
  created_at: string
  updated_at: string
}

/** Payload for POST /api/v1/secrets (create or update) */
export interface SecretInput {
  id: string
  name: string
  type: SecretType
  /** Key/value credential pairs — encrypted server-side; never exposed after creation */
  value: Record<string, string>
  metadata?: Record<string, string>
}
