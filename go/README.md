# mrlib - Mistral AI Libraries CLI

A fast, standalone Go CLI for managing Mistral AI Libraries and Documents with two-way file synchronization.

## Installation

### Build from Source

```bash
cd go
go build -o mrlib ./cmd/main.go
```

The binary will be created at `mrlib`.

### Using Make

```bash
make build
```

### Cross-Compilation

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o mrlib-linux ./cmd/main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o mrlib-mac ./cmd/main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o mrlib-mac-arm64 ./cmd/main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o mrlib.exe ./cmd/main.go
```

## Quick Start

```bash
# Set your API key
export MISTRAL_API_KEY=your_api_key_here

# Or use --api-key flag
mrlib --api-key your_api_key_here lib list
```

## Usage

### Libraries (`lib`)

```bash
# List all libraries
mrlib lib list

# Get a specific library (by ID or name)
mrlib lib get 019cb08e-95b1-7140-9793-cc464373162c
mrlib lib get "My Library Name"

# Create a new library
mrlib lib create "My Library" --description "My documents"

# Update a library
mrlib lib update "Old Name" "New Name" --description "Updated"

# Delete a library
mrlib lib delete "Library Name" --force
```

### Documents (`doc`)

```bash
# List documents in a library (by ID or name)
mrlib doc list 019cb08e-95b1-7140-9793-cc464373162c
mrlib doc list "My Library Name"

# Download a document
mrlib doc get "Library Name" DOCUMENT_ID --output downloaded.pdf

# Upload a file
mrlib doc upload "Library Name" --file path/to/file.pdf

# Delete a document
mrlib doc delete "Library Name" DOCUMENT_ID --force
```

### Sync

```bash
# One-time sync (dry run first to preview)
mrlib sync once "Library Name" /path/to/local/folder --dry-run
mrlib sync once "Library Name" /path/to/local/folder

# Continuous sync (every 60 seconds)
mrlib sync continuous "Library Name" /path/to/local/folder --interval 60

# Check sync status
mrlib sync status
```

## Sync Options

- `--direction`: `up` (local→remote), `down` (remote→local), or `both` (default)
- `--mode`: `mirror` (exact sync), `additive` (only add), or `safe` (add+update, no delete)
- `--dry-run`: Preview changes without applying
- `--force`: Force re-upload even if hash matches
- `--extensions`: File extensions to include (e.g., `pdf,txt`)
- `--exclude`: Patterns to exclude
- `--include`: Patterns to include
- `--batch-size`: Batch size for uploads (default: 10)
- `--max-workers`: Maximum parallel workers (default: 4)
- `--state-file`: Path to sync state file (default: `.mistral_sync_state.json`)

## Configuration

Create a `mrlib.yaml` file:

```yaml
api_key: YOUR_API_KEY
base_url: https://api.mistral.ai/v1
timeout: 120
rate_limit_delay: 1.0
max_retries: 3
```

Then run:
```bash
mrlib --config mrlib.yaml lib list
```

## Features

- ✅ List, get, create, update, delete libraries
- ✅ List, get, upload, delete documents
- ✅ Library name resolution (use names instead of UUIDs)
- ✅ Two-way synchronization (up, down, both)
- ✅ Multiple sync modes (mirror, additive, safe)
- ✅ Dry-run mode for previewing changes
- ✅ File filtering by extension and patterns
- ✅ Rate limiting and retry logic
- ✅ JSON output for scripting (`--json`)
- ✅ Configuration file support
- ✅ Environment variable support (`MISTRAL_API_KEY`)
- ✅ Caching for library name→ID resolution

## Examples

### Input/Output Behavior

**Libraries list:**
```bash
$ mrlib lib list
ID                                   Name                 Docs     Size      
--------------------------------------------------------------------------------
019cb08e-95b1-7140-9793-cc464373162c tvb-ins-amu Library  1        14.7 MB
019cae7e-66bd-77f4-b41c-947925975128 vb-tech              6        2.0 MB
```

**Documents list with library name:**
```bash
$ mrlib doc list "tvb-ins-amu Library"
ID                                   Name                 Size       Status      
--------------------------------------------------------------------------------
9c95c9e9-15f1-40e0-ab1f-b866e291d7dc main.pdf             14.7 MB
```

**JSON output:**
```bash
$ mrlib lib list --json
[{"id":"019cb08e...","name":"tvb-ins-amu Library","nb_documents":1,...}]
```

**Sync status:**
```bash
$ mrlib sync status
Sync Status
===========
Library:   tvb-ins-amu Library (019cb08e-95b1-7140-9793-cc464373162c)
Local:     /path/to/local
Last Sync: 2026-05-11 17:17:31

Pending Changes:
  To Upload:   0 files
  To Download: 0 files
  Modified:    1 files
    ~ main.pdf
  In Sync:     0 files
```

## Project Structure

```
go/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── client.go        # HTTP client with retries & caching
│   │   └── client_test.go   # Unit tests
│   ├── cli/
│   │   └── cli.go           # Cobra-based CLI commands
│   ├── config/
│   │   └── config.go        # Configuration file loading
│   ├── models/
│   │   └── models.go        # Data models
│   └── sync/
│       └── sync.go          # Two-way sync engine
├── go.mod
├── go.sum
├── README.md
└── .gitignore
```

## Requirements

- Go 1.19+

## License

MIT License
