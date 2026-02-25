-- =============================================================================
-- flowjs-works — Reference Database Schema
-- =============================================================================
-- This file is the canonical reference for the full PostgreSQL schema.
-- Deployed scripts live in /init-db/. Keep both in sync.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- DATABASE: flowjs_config  (Control Plane — process definitions & secrets)
-- ---------------------------------------------------------------------------

-- Processes table: stores the JSON DSL for each flow
CREATE TABLE IF NOT EXISTS processes (
    id            VARCHAR(255) PRIMARY KEY,       -- matches definition.id
    version       VARCHAR(50)  NOT NULL,
    name          VARCHAR(255) NOT NULL,
    description   TEXT,
    dsl           JSONB        NOT NULL,          -- full FlowDSL document
    status        VARCHAR(20)  DEFAULT 'draft',   -- draft | deployed | stopped
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_processes_status ON processes (status);

-- Secrets table: encrypted credentials referenced by nodes via secret_ref
CREATE TABLE IF NOT EXISTS secrets (
    id            VARCHAR(255) PRIMARY KEY,       -- e.g. sec_postgres_main
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(50)  NOT NULL,          -- basic_auth | token | certificate | connection_string
    encrypted_val BYTEA        NOT NULL,          -- AES-256-GCM encrypted JSON blob
    metadata      JSONB,                          -- non-sensitive labels/tags
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ---------------------------------------------------------------------------
-- DATABASE: flowjs_audit  (Data Plane — execution history)
-- ---------------------------------------------------------------------------

-- Executions table: one row per flow invocation
CREATE TABLE IF NOT EXISTS executions (
    execution_id       UUID PRIMARY KEY,
    flow_id            VARCHAR(255) NOT NULL,
    version            VARCHAR(50),
    status             VARCHAR(20),                -- STARTED | COMPLETED | FAILED | REPLAYED
    correlation_id     VARCHAR(255),
    start_time         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    end_time           TIMESTAMP WITH TIME ZONE,
    trigger_type       VARCHAR(50),
    main_error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_exec_flow     ON executions (flow_id);
CREATE INDEX IF NOT EXISTS idx_exec_corr     ON executions (correlation_id);
CREATE INDEX IF NOT EXISTS idx_exec_status   ON executions (status);

-- Activity logs table: one row per node execution
CREATE TABLE IF NOT EXISTS activity_logs (
    log_id        BIGSERIAL PRIMARY KEY,
    execution_id  UUID REFERENCES executions(execution_id),
    node_id       VARCHAR(255) NOT NULL,
    node_type     VARCHAR(50),
    status        VARCHAR(20),                     -- SUCCESS | ERROR
    input_data    JSONB,
    output_data   JSONB,
    error_details JSONB,
    duration_ms   INTEGER,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_activity_exec   ON activity_logs (execution_id);
CREATE INDEX IF NOT EXISTS idx_activity_input   ON activity_logs USING GIN (input_data);
CREATE INDEX IF NOT EXISTS idx_activity_output  ON activity_logs USING GIN (output_data);
