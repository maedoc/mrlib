# AI Agents Documentation

This project was developed with AI-assisted workflows. This document describes the agent setup and development process.

## Agent Configuration

- **Model**: mistral-medium-3.5 (custom provider)
- **Capabilities**: Full tool access, filesystem, external APIs
- **Subagents**: Spawned for parallel work (code review, research, independent tasks)

## Development Process

### Phase 1: Project Setup
- Created Go module structure: `cmd/`, `internal/{api,cli,config,models,sync}/`
- Added dependencies: cobra, viper, yaml
- Set up build system with Makefile

### Phase 2: Core Implementation

**Models** (`go/internal/models/models.go`):
- Library, Document, SyncConfig, SyncState structs
- Enum types for SyncDirection, SyncMode, DocumentStatus

**API Client** (`go/internal/api/client.go`):
- HTTP client with exponential backoff retry
- Library name→ID caching (5 min TTL)
- CRUD operations for libraries and documents
- Signed URL download support

**Sync Engine** (`go/internal/sync/sync.go`):
- Two-way file synchronization (up/down/both)
- Three modes: mirror, additive, safe
- State file persistence with conflict detection
- File filtering by extension and patterns

**CLI** (`go/internal/cli/cli.go`):
- Cobra-based command structure
- Short aliases: `lib`, `doc`, `sync`
- Library name resolution (auto-lookup UUIDs)
- JSON output for scripting

**Config** (`go/internal/config/config.go`):
- YAML config file with env var override
- CLI flag precedence (flags > env > file)

### Phase 3: Testing & Refinement
- Unit tests with mocked HTTP server (`go/internal/api/client_test.go`)
- Performance: stripped binary (`-ldflags="-s -w"`), 9.2MB → 6.4MB
- UX: name resolution, short commands, binary renamed from `mistral-file-sync` to `mrlib`

## Build System

| Command | Description |
|---------|-------------|
| `make build` | Build for current platform |
| `make test` | Run Go tests |
| `make clean` | Clean artifacts |
| `make build-all` | Cross-compile all platforms |
| `make install` | Build and install to `/usr/local/bin` |

## Key Design Decisions

- **No Python dependency**: Pure Go binary, single static executable
- **Caching**: In-memory library name→ID cache with 5 min TTL
- **Rate limiting**: Configurable delay between API requests
- **Parallelism**: Configurable max workers for sync operations

## Caching Strategy

| Cache | TTL | Storage |
|-------|-----|---------|
| Library name → ID | 5 min | In-memory map |

### Planned (not yet implemented)
- Document list cache (per-library)
- HTTP ETag/Last-Modified caching
- File hash cache (skip unchanged uploads)

## License

MIT
