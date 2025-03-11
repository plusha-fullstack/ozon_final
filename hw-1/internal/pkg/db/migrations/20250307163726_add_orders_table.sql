-- +goose Up
CREATE TABLE orders (
    id TEXT PRIMARY KEY,
    recipient_id TEXT NOT NULL,
    storage_until TIMESTAMP NOT NULL,
    status TEXT NOT NULL,
    price INTEGER NOT NULL,
    weight REAL NOT NULL,
    wrapper TEXT NOT NULL CHECK (wrapper IN ('Bag', 'Box', 'Membrane', 'Bag with Membrane', 'Box with Membrane')),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE orders;