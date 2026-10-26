# Flare P-Chain Indexer

Blockchain indexing suite for the Avalanche P-chain of Flare networks. Two executables: **indexer** (indexes P-chain blocks/txs into MySQL) and **services** (REST API for querying indexed data).

## Build & Run

```bash
# Build
go build -o flare_indexer ./indexer/main/indexer.go
go build -o flare_services ./services/main/services.go

# Run
./flare_indexer --config config.local.toml
./flare_services --config config.local.toml

# Docker
docker build -f dockerfile.indexer -t flare_indexer:latest .
docker build -f dockerfile.services -t flare_services:latest .
```

## Testing

```bash
# Start test database first
cd resources/test && docker-compose up -d

# Run all tests
go test ./...

# Run specific package tests
go test ./database/...
go test ./utils/...
```

Test database: MySQL on port 3307, database `flare_indexer_services`, user `indexeruser`.

## Code Style

- Standard `gofmt` formatting (check with `./gofmt-check.sh`)
- No additional linter configured
- CamelCase for functions/variables, SCREAMING_SNAKE_CASE for constants
- Standard Go error returns, no exceptions

## Project Structure

| Directory | Purpose |
|-----------|---------|
| `indexer/` | P-chain indexer, cronjobs, migrations (main executable) |
| `services/` | REST API server (second executable) |
| `database/` | GORM entities, queries, DB utilities |
| `utils/` | Shared utilities (chain, merkle, staking, contracts) |
| `config/` | Global config structures (TOML + env vars) |
| `logger/` | Centralized logging with zap |
| `docker/` | Docker compose and deployment configs |

## Key Files

- `indexer/main/indexer.go` - Indexer entry point
- `services/main/services.go` - Services entry point
- `database/entities.go` - Database entity definitions
- `database/pchain_queries.go` - P-chain database queries
- `database/utils.go` - DB connection, auto-migration, transactions
- `config/config.go` - Configuration structures
- `indexer/migrations/container.go` - Custom migration system

## Tech Stack

- **Go 1.26** with **GORM** (MySQL)
- **Avalanche Go** v1.15.1 (Helicon: auto-renewed validators, ACP-236) for chain interaction
- **ava-labs/libevm** (go-ethereum fork used by avalanchego/coreth) for smart contract interaction
- **gorilla/mux** for HTTP routing
- **zap** for structured logging
- **Prometheus** for metrics

## Configuration

TOML config files (default `config.toml`, override with `--config`). Environment variables supported (e.g., `DB_HOST`, `DB_PASSWORD`, `CHAIN_NODE_URL`). Per-network configs exist for flare, coston, coston2, avalanche.

## Database

MySQL with GORM. Auto-migration on startup. Custom versioned migration system via `migrations.Container.Add()` with timestamp-based ordering (e.g., "2023-08-25-00-00"). Transaction helper: `database.DoInTransaction()`.