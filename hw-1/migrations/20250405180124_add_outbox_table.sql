-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS outbox_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status task_status NOT NULL DEFAULT 'CREATED',
    payload JSONB NOT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT 'audit_logs',
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE outbox_tasks;
-- +goose StatementEnd
