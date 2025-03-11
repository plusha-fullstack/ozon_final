-- +goose Up
CREATE TABLE order_history(
    id SERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    changed_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE history_entries;