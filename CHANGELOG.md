# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-25

### Added

#### CLI Commands
- `otacon scan` — Full cluster health scan with A-F scorecard (50+ rules, 5 categories)
- `otacon audit` — Best practice compliance check with `--explain` mode
- `otacon diagnose network` — CoreDNS, endpoint, and NetworkPolicy diagnostics
- `otacon diagnose logs` — CrashLoopBackOff and OOMKill detection with log analysis
- `otacon diagnose nodes` — Node readiness, pressure conditions, capacity analysis
- `otacon resources` — Resource configuration analysis and right-sizing
- `otacon events` — Event timeline with `--correlate` for incident grouping
- `otacon guardian start` — In-cluster daemon mode with full pipeline

#### Intelligence Engine
- Real-time Kubernetes event watcher with severity classification (30+ patterns)
- Event correlation engine with 7 built-in rules (node cascade, OOM cascade, image pull chain, volume mount chain, scheduling pressure, DNS failure, probe cascade)
- Smart deduplication with configurable time windows and group-by fields
- Cooldown manager with per-severity durations and safety valve
- Periodic digest builder (daily/weekly) with trending analysis

#### Notification System
- Multi-channel notification router with rule-based routing
- Slack channel with rich Block Kit messages and digest formatting
- Microsoft Teams webhook integration
- Email (SMTP) with HTML templates
- Generic webhook channel
- Enrichment plugins: Log enricher, Grafana link enricher, Runbook enricher
- Alertmanager webhook receiver (bidirectional bridge)

#### Web UI
- React dashboard with dark terminal aesthetic
- Dashboard: Score trend chart, stat cards, category breakdown, recent incidents
- Events: Real-time timeline (10s polling), severity/time filters, dedup groups
- Audit: Scorecard display, expandable findings browser, HTML/PDF export
- Resources: Missing limits/requests summary, findings table
- Status: System health, storage stats, notification metrics, cooldown stats
- Embedded into Go binary via `embed.FS`

#### Storage
- SQLite with WAL mode for concurrent reads
- Tables: events, incidents, audit_reports, digests
- Configurable retention (events: 7d, audits: 7d, digests: 30d)
- Hourly auto-cleanup worker

#### API Server
- REST endpoints: /events, /incidents, /audit/reports, /audit/history, /dedup/groups, /stats, /status
- Health endpoints: /healthz, /readyz
- Alertmanager webhook receiver: /api/v1/alertmanager/webhook
- HTML report export
- CORS enabled

#### DevOps & Release
- Multi-stage Dockerfile (distroless, non-root)
- GoReleaser for multi-platform builds (linux/darwin/windows, amd64/arm64)
- GitHub Actions CI (lint, test, build, security scan)
- GitHub Actions Release pipeline
- Helm chart with RBAC, PVC, Ingress support
- Makefile with build, test, lint, docker, helm targets
- golangci-lint configuration (15+ linters)

#### Documentation
- README with badges, installation, usage examples, architecture diagram
- Getting Started guide
- CLI Reference (all commands and flags)
- Architecture documentation with data flow diagrams
- Audit Rules Reference (all 50+ rules)
- CRD Reference (NotificationRule, AuditPolicy, DigestSchedule)
- Architecture Decision Records (ADR-001, ADR-002)
- CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, LICENSE (Apache 2.0)
- GitHub issue templates (Bug Report, Feature Request)

#### Testing
- Unit tests for core types and grading
- Unit tests for event watcher severity classification
- Unit tests for correlation engine, deduplicator, cooldown manager
- Unit tests for notification router with mock channels
- Unit tests for SQLite store (CRUD, filters, stats)
- Unit tests for API server handlers and middleware
