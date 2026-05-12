# AI Agents Documentation

This document describes the AI agents and automation used in this project.

## Project Agents

### Primary Agent (Hermes)
- **Role**: Research engineer assistant
- **Capabilities**: Full access to tools, file system, and external APIs
- **Model**: Custom configured (mistral-medium-3.5)
- **Provider**: Custom

### Subagents
- **Spawned via**: `delegate_task` for parallel work
- **Constraints**: No nesting (max_spawn_depth=1)
- **Use cases**:
  - Code review
  - Research synthesis
  - Parallel independent tasks

## Go Project Development

The Go CLI (`mrlib`) was developed with the following agent workflow:

### Phase 1: Project Setup
1. Created directory structure: `go/cmd/`, `go/internal/{api,cli,config,models,sync}/`
2. Initialized Go module: `go mod init mistral-file-sync`
3. Added dependencies: cobra, viper, yaml

### Phase 2: Core Implementation
1. **Models** (`internal/models/models.go`):
   - Library, Document, SyncConfig, SyncState structs
   - Type definitions for enums (SyncDirection, SyncMode, etc.)

2. **API Client** (`internal/api/client.go`):
   - HTTP client with retry logic
   - Library name → ID caching (5 min TTL)
   - All CRUD operations for libraries and documents
   - Signed URL handling for downloads

3. **Sync Engine** (`internal/sync/sync.go`):
   - Two-way file synchronization
   - Multiple sync modes (mirror, additive, safe)
   - State file persistence
   - Conflict detection

4. **CLI** (`internal/cli/cli.go`):
   - Cobra-based command structure
   - Short command names: `lib`, `doc`, `sync`
   - Library name resolution support
   - JSON output for scripting

5. **Config** (`internal/config/config.go`):
   - YAML configuration file support
   - Environment variable override
   - Command-line flag precedence

### Phase 3: Testing & Refinement
1. Added unit tests (`internal/api/client_test.go`):
   - Test server mocking
   - 5 tests covering core API functionality

2. Performance optimizations:
   - Library name caching
   - Stripped binary (`-ldflags="-s -w"`)
   - Reduced from 9.2MB to 6.4MB

3. UX improvements:
   - Library name → ID resolution
   - Short command names
   - Binary renamed from `mistral-file-sync` to `mrlib`

## Build System

### Makefile Targets
```bash
make build      # Build the mrlib binary
make test       # Run Go tests
make clean      # Clean build artifacts
make build-linux    # Cross-compile for Linux
make build-mac      # Cross-compile for macOS
make build-windows  # Cross-compile for Windows
```

### Version Info
- Go: 1.19+
- Dependencies:
  - github.com/spf13/cobra v1.7.0
  - github.com/spf13/viper v1.17.0
  - gopkg.in/yaml.v3 v3.0.1

## Usage Patterns

### Common Workflows

**List libraries:**
```bash
mrlib lib list
```

**List documents with library name:**
```bash
mrlib doc list "My Library Name"
```

**Sync files:**
```bash
mrlib sync once "Library Name" ./local-folder
```

**Check sync status:**
```bash
mrlib sync status
```

### Configuration

**Environment variable:**
```bash
export MISTRAL_API_KEY=your_key
mrlib lib list
```

**Config file:**
```bash
mrlib --config mrlib.yaml lib list
```

**Command-line flag:**
```bash
mrlib --api-key your_key lib list
```

## Caching Strategy

### Library Name → ID Cache
- **TTL**: 5 minutes
- **Trigger**: First `ListLibraries()` call or direct lookup
- **Storage**: In-memory map in Client struct
- **Fallback**: Direct API lookup on cache miss

### Future Cache Enhancements (not implemented)
1. **Document list cache**: Cache per-library document listings
2. **HTTP response cache**: Use ETag/Last-Modified headers
3. **File hash cache**: Skip unchanged file uploads
4. **Connection pooling**: Already optimized via http.Client

## File Structure

```
.
├── go/
│   ├── cmd/
│   │   └── main.go          # Entry point (calls cli.Execute())
│   ├── internal/
│   │   ├── api/
│   │   │   ├── client.go    # API client with caching
│   │   │   └── client_test.go # Unit tests
│   │   ├── cli/
│   │   │   └── cli.go       # All CLI commands
│   │   ├── config/
│   │   │   └── config.go    # Config file loading
│   │   ├── models/
│   │   │   └── models.go    # Data structures
│   │   └── sync/
│   │       └── sync.go      # Sync logic
│   ├── go.mod
│   ├── go.sum
│   ├── README.md            # This file
│   └── .gitignore
├── Makefile                  # Build automation
└── mrlib                    # Compiled binary
```

## API Endpoints Used

- `GET /api/v1/libraries` - List libraries
- `POST /api/v1/libraries` - Create library
- `GET /api/v1/libraries/{id}` - Get library
- `PUT /api/v1/libraries/{id}` - Update library
- `DELETE /api/v1/libraries/{id}` - Delete library
- `GET /api/v1/libraries/{id}/documents` - List documents
- `POST /api/v1/libraries/{id}/documents` - Upload document
- `GET /api/v1/libraries/{id}/documents/{doc_id}` - Get document
- `DELETE /api/v1/libraries/{id}/documents/{doc_id}` - Delete document
- `GET /api/v1/libraries/{id}/documents/{doc_id}/download` - Download document

## Authentication

- **Header**: `Authorization: Bearer {api_key}`
- **Sources** (priority order):
  1. Command-line flag (`--api-key`)
  2. Config file (`api_key`)
  3. Environment variable (`MISTRAL_API_KEY`)

## Error Handling

- **Retry logic**: Exponential backoff on rate limits (429)
- **Authentication errors**: Clear error messages for 401/403
- **Network errors**: Configurable timeout and max retries
- **Validation**: Input validation for library names, file paths

## Performance Considerations

- **Rate limiting**: Configurable delay between requests (default: 1s)
- **Parallelism**: Configurable max workers for sync operations (default: 4)
- **Batching**: Configurable batch size for uploads (default: 10)
- **Caching**: Library name resolution cached for 5 minutes

## Testing

Run tests:
```bash
cd go
go test ./...
```

Or via Make:
```bash
make test
```

## License

This project is licensed under the MIT License.
