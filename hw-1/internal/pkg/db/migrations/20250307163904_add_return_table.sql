-- +goose Up
CREATE TABLE returns (
    id SERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    returned_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE returns;