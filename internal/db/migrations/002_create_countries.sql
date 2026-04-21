-- +goose Up
CREATE TABLE IF NOT EXISTS countries (
    country TEXT PRIMARY KEY,
    region  TEXT NOT NULL,
    iso3    CHAR(3)
);

-- +goose Down
DROP TABLE IF EXISTS countries;
