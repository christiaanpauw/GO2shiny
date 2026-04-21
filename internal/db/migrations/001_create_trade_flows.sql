-- +goose Up
CREATE TABLE IF NOT EXISTS trade_flows (
    id          BIGSERIAL PRIMARY KEY,
    year        SMALLINT      NOT NULL,
    quarter     CHAR(2),
    country     TEXT          NOT NULL,
    region      TEXT,
    type_ie     TEXT          NOT NULL,
    type_gs     TEXT          NOT NULL,
    commodity   TEXT,
    hs_code     TEXT,
    value_nzd   NUMERIC(18,3) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trade_flows_year    ON trade_flows (year);
CREATE INDEX IF NOT EXISTS idx_trade_flows_country ON trade_flows (country);
CREATE INDEX IF NOT EXISTS idx_trade_flows_type_ie ON trade_flows (type_ie);

-- +goose Down
DROP TABLE IF EXISTS trade_flows;
