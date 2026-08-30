-- +goose Up
CREATE TABLE metrics (
    key   TEXT             NOT NULL UNIQUE,
    type  TEXT             NOT NULL,
    value DOUBLE PRECISION,
    delta BIGINT
);

-- +goose Down
DROP TABLE metrics;
