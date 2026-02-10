-- Starter migration placeholder for {{PROJECT_NAME}}.
-- Replace with schema generated from your CUE entities if needed.

CREATE TABLE IF NOT EXISTS _ang_bootstrap (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
