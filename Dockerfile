# ── Migrate stage ─────────────────────────────────────────────────────────────
# Installs the goose CLI and bundles the SQL migration files.
# Used as a one-shot init container in docker-compose.yml.
FROM golang:1.24-alpine AS migrate

RUN go install github.com/pressly/goose/v3/cmd/goose@v3.24.3

COPY internal/db/migrations /migrations

ENTRYPOINT ["goose", "-dir", "/migrations", "postgres"]
CMD ["up"]

# ── Importer stage ─────────────────────────────────────────────────────────────
# Builds the import_data CLI and bundles the CSV seed files.
# Used as a one-shot init container in docker-compose.yml.
FROM golang:1.24-alpine AS importer

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/import_data \
    ./scripts/import_data

ENTRYPOINT ["/bin/import_data"]
CMD ["-trade", "data/sample/trade_flows.csv", "-countries", "data/sample/countries.csv"]

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Cache module downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/server \
    ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: needed for outbound TLS (e.g. CDN resources in templates).
# tzdata: needed if the server ever renders localised timestamps.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/server /bin/server

EXPOSE 8080

# Run as a non-root user for defence-in-depth.
USER nobody

ENTRYPOINT ["/bin/server"]
