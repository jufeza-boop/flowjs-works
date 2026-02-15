-- 1. Tabla de Ejecuciones (Cabecera)
CREATE TABLE executions (
    execution_id UUID PRIMARY KEY,
    flow_id VARCHAR(255) NOT NULL, -- ID del proceso en el DSL
    version VARCHAR(50),           -- Versión del DSL ejecutada
    status VARCHAR(20),            -- STARTED, COMPLETED, FAILED, REPLAYED
    correlation_id VARCHAR(255),    -- ID para trazar el mensaje entre sistemas
    start_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    end_time TIMESTAMP WITH TIME ZONE,
    trigger_type VARCHAR(50),       -- webhook, cron, sftp, etc.
    main_error_message TEXT
);

-- 2. Tabla de Logs de Actividad (Detalle de cada Nodo)
CREATE TABLE activity_logs (
    log_id BIGSERIAL PRIMARY KEY,
    execution_id UUID REFERENCES executions(execution_id),
    node_id VARCHAR(255) NOT NULL, -- ID del nodo en el DSL
    node_type VARCHAR(50),
    status VARCHAR(20),            -- SUCCESS, ERROR
    input_data JSONB,              -- El payload exacto que recibió el nodo
    output_data JSONB,             -- El payload exacto que generó el nodo
    error_details JSONB,           -- Stack trace o código de error si falló
    duration_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Índices GIN para búsquedas rápidas dentro del JSON
CREATE INDEX idx_activity_input ON activity_logs USING GIN (input_data);
CREATE INDEX idx_activity_output ON activity_logs USING GIN (output_data);
CREATE INDEX idx_correlation_id ON executions (correlation_id);