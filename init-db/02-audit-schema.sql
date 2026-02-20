-- Connect to the audit database and create schema
\c flowjs_audit;

-- 1. Tabla de Ejecuciones (Cabecera)
CREATE TABLE IF NOT EXISTS executions (
    execution_id UUID PRIMARY KEY,
    flow_id VARCHAR(255) NOT NULL,
    version VARCHAR(50),
    status VARCHAR(20),            -- STARTED, COMPLETED, FAILED, REPLAYED
    correlation_id VARCHAR(255),
    start_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    end_time TIMESTAMP WITH TIME ZONE,
    trigger_type VARCHAR(50),
    main_error_message TEXT
);

-- 2. Tabla de Logs de Actividad (Detalle de cada Nodo)
CREATE TABLE IF NOT EXISTS activity_logs (
    log_id BIGSERIAL PRIMARY KEY,
    execution_id UUID REFERENCES executions(execution_id),
    node_id VARCHAR(255) NOT NULL,
    node_type VARCHAR(50),
    status VARCHAR(20),            -- SUCCESS, ERROR
    input_data JSONB,
    output_data JSONB,
    error_details JSONB,
    duration_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Índices GIN para búsquedas rápidas dentro del JSON
CREATE INDEX IF NOT EXISTS idx_activity_input ON activity_logs USING GIN (input_data);
CREATE INDEX IF NOT EXISTS idx_activity_output ON activity_logs USING GIN (output_data);
CREATE INDEX IF NOT EXISTS idx_correlation_id ON executions (correlation_id);
