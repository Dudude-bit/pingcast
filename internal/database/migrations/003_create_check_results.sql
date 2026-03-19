CREATE TABLE check_results (
    id BIGSERIAL PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status VARCHAR(10) NOT NULL,
    status_code INT,
    response_time_ms INT NOT NULL,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_check_results_monitor_checked ON check_results (monitor_id, checked_at);
