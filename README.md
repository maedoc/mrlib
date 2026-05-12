# mrlib — Mistral AI Libraries CLI

Fast, standalone Go CLI for managing Mistral AI Libraries and Documents with two-way file synchronization.

## Installation

```bash
git clone git@github.com:maedoc/mrlib.git
cd mrlib
make build
```

Or build manually:

```bash
cd go
go build -o mrlib ./cmd/main.go
```

## Quick Start

```bash
export MISTRAL_API_KEY=your_api_key_here
./mrlib lib list
```

## Usage

### Libraries (`lib`)

```bash
mrlib lib list                              # List all libraries
mrlib lib get "My Library"                  # Get by name or ID
mrlib lib create "My Library"               # Create new library
mrlib lib update "Old" "New"               # Rename library
mrlib lib delete "My Library" --force       # Delete library
```

### Documents (`doc`)

```bash
mrlib doc list "My Library"                 # List documents
mrlib doc upload "My Library" --file doc.pdf
mrlib doc get "My Library" DOC_ID --output doc.pdf
mrlib doc delete "My Library" DOC_ID --force
```

### Sync

```bash
mrlib sync once "My Library" ./folder       # One-time sync
mrlib sync once "My Library" ./folder --dry-run  # Preview first
mrlib sync continuous "My Library" ./folder --interval 60  # Watch mode
mrlib sync status                           # Check sync state
```

## Configuration

Priority: CLI flags > env vars > config file.

```bash
export MISTRAL_API_KEY=your_key
export MISTRAL_BASE_URL=https://api.mistral.ai/v1
```

Or create a `mrlib.yaml`:

```yaml
api_key: YOUR_API_KEY
base_url: https://api.mistral.ai/v1
timeout: 120
```

## Sync Modes

| Mode      | Upload | Download | Delete |
|-----------|--------|----------|--------|
| `mirror`  | yes    | yes      | yes    |
| `additive`| yes    | yes      | no     |
| `safe`    | yes    | yes      | no     |

## Project Structure

```
├── go/
│   ├── cmd/main.go              # Entry point
│   ├── internal/
│   │   ├── api/client.go        # HTTP client with retries & caching
│   │   ├── cli/cli.go           # Cobra-based CLI
│   │   ├── config/config.go     # YAML/env config loading
│   │   ├── models/models.go     # Data models
│   │   └── sync/sync.go         # Two-way sync engine
│   ├── go.mod / go.sum
│   └── go/README.md
├── Makefile                      # Build automation
├── AGENTS.md                     # AI agent documentation
└── README.md                     # This file
```

## Requirements

- Go 1.19+

## License

MIT
