# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is zk

A plain text note-taking CLI assistant designed for Zettelkasten / personal wiki workflows. Written in Go, backed by SQLite with FTS5 for full-text search.

## Build & Test

Requires Go 1.21+ and CGO (for SQLite). Always use `make` — it sets required flags (`CGO_ENABLED=1`, `-tags "fts5"`, version ldflags).

```bash
make build          # Build ./zk binary
make install        # Build and install to $GOPATH/bin
make test           # Unit tests + gofmt
make tesh           # End-to-end tests (builds first, requires tesh CLI)
make tesh-update    # Update e2e test expectations after output changes
```

Run a single unit test: `CGO_ENABLED=1 go test -tags "fts5" -run TestName ./internal/core/`

## Architecture

Hexagonal / Ports & Adapters pattern:

- **`internal/core/`** — Domain logic and interfaces (ports). Notebook, Note, Config, Link, Collection entities. This is where business rules live.
- **`internal/adapter/`** — Implementations of core interfaces:
  - `sqlite/` — Note indexing and search (NoteIndex)
  - `markdown/` — Note content parsing
  - `handlebars/` — Template rendering (raymond library)
  - `lsp/` — Language Server Protocol server
  - `fzf/` — Interactive fuzzy finder integration
  - `editor/` — External editor launching
  - `fs/` — File system operations
  - `term/` — Terminal styling
- **`internal/cli/`** — Kong-based CLI layer:
  - `cmd/` — One file per command (init, new, list, edit, index, graph, tag, lsp)
  - `container.go` — Dependency injection container wiring adapters to core
  - `filtering.go` — Shared filter/query flag parsing
- **`internal/util/`** — Shared utilities (date parsing, FTS5 query building, paths, optional types)

**Entry point:** `main.go` → parses flags → creates DI container → discovers notebook → auto-indexes → runs command.

**Notebook discovery order:** `--notebook-dir` flag → current directory walk → `ZK_NOTEBOOK_DIR` env var.

## Branching

Branch from `dev`, not `main`. PRs target `dev`.

## Testing Conventions

- Unit tests: standard Go `testing` library, co-located `*_test.go` files
- E2E tests: `tesh` framework, files in `tests/*.tesh` with fixtures in `tests/fixtures/`
- When fixing a GitHub issue, create `tests/issue-XXX.tesh` first
- `make test` also runs `gofmt` on all `.go` files (formatting is enforced)

## Key Dependencies

- `alecthomas/kong` — CLI parsing
- `mattn/go-sqlite3` — SQLite driver (CGO, FTS5)
- `aymerick/raymond` — Handlebars templates
- `yuin/goldmark` — Markdown parsing
- `tliron/glsp` — LSP protocol implementation
