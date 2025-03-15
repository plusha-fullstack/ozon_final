-- +goose Up

CREATE TYPE wrapper_type AS ENUM ('Bag', 'Box', 'Membrane', 'Bag with Membrane', 'Box with Membrane');

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    recipient_id TEXT NOT NULL,
    storage_until TIMESTAMP NOT NULL,
    status TEXT NOT NULL,
    price INTEGER NOT NULL,
    weight REAL NOT NULL,
    wrapper wrapper_type NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
-- +goose Down
DROP TABLE orders;