# Otacon Architecture

## Overview
Otacon is an event-driven intelligence platform for Kubernetes operating in CLI and Guardian (daemon) modes.

## Components
- **Audit Scanner**: 50+ rules across 5 categories, weighted A-F scorecard
- **Event Watcher**: Real-time K8s event stream via watch API
- **Correlator**: 7 built-in patterns (node cascade, OOM, image pull, volume, scheduling, DNS, probe failures)
- **Deduplicator**: Groups by namespace+reason, threshold-based emission
- **Cooldown Manager**: Per-group throttling with severity-based durations and safety valve
- **Notification Router**: Rule-based routing with enrichment pipeline (Slack, Teams, Email, Webhook)
- **SQLite Storage**: WAL mode, auto-retention, events/incidents/audits/digests tables
- **HTTP API**: REST endpoints for UI, Alertmanager webhook receiver
- **Web UI**: React SPA embedded into Go binary

## Scoring Formula
Category weight: Security 25%, Resources 20%, Reliability 25%, Best Practices 15%, Network 15%
Per-finding deduction: Critical -10 (max -60), Warning -5 (max -30), Info -2 (max -10)
Overall = weighted sum of category scores (0-100)

## Key Decisions
1. Single binary — CLI + daemon + API + UI embedded
2. SQLite (WAL) — no external DB dependency
3. Event-driven — watch API, not polling
4. Correlate before notify — intelligence pipeline reduces noise
