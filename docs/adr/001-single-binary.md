# ADR-001: Single Binary Architecture

## Status
Accepted

## Context
Otacon needs to be easy to install and deploy. We need to decide between:
1. Single binary with embedded UI
2. Separate binaries for CLI and server
3. Container-only deployment

## Decision
Single binary with all components embedded (CLI, API server, Web UI, SQLite).

## Rationale
- **Zero dependencies**: No external database, message queue, or UI server needed
- **Simple deployment**: Download one file, run it
- **Helm compatibility**: Single container image with configurable flags
- **Developer experience**: `go install` or `brew install` gets everything
- **Consistent versioning**: All components always match

## Consequences
- Binary size is larger (~30-50MB) due to embedded UI assets
- SQLite limits concurrent write throughput (acceptable for single-instance)
- Optional PostgreSQL support planned for multi-instance deployments

## Alternatives Considered
- Separate CLI/server: Rejected due to version coordination complexity
- etcd/Redis storage: Rejected due to operational overhead for end users
