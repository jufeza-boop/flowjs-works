-- Connect to the config database and create the control-plane schema
\c flowjs_config;

-- ---------------------------------------------------------------------------
-- Processes table: stores the JSON DSL for each flow
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS processes (
    id          VARCHAR(255) PRIMARY KEY,
    version     VARCHAR(50)  NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    dsl         JSONB        NOT NULL,
    status      VARCHAR(20)  DEFAULT 'draft',  -- draft | deployed | stopped
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_processes_status ON processes (status);

-- ---------------------------------------------------------------------------
-- Secrets table: AES-256-GCM encrypted credentials referenced by nodes
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS secrets (
    id            VARCHAR(255) PRIMARY KEY,       -- e.g. sec_postgres_main
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(50)  NOT NULL,          -- basic_auth | token | certificate | connection_string
    encrypted_val BYTEA        NOT NULL,          -- AES-256-GCM encrypted JSON blob
    metadata      JSONB,                          -- non-sensitive labels/tags
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
