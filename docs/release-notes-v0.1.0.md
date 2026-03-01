# Otacon v0.1.0 — Foundation Release

> Intelligent Kubernetes Diagnostics, Audit & Event Correlation Platform

Otacon is a single-binary tool that brings intelligence to Kubernetes operations. Instead of forwarding thousands of raw events, Otacon correlates them into actionable incidents, scores your cluster health, and delivers smart notifications — so you can focus on what matters.

---

## Highlights

### Cluster Health Scorecard
Comprehensive A-F grading across 5 categories with 50+ audit rules. Know your cluster's security posture, resource efficiency, and reliability at a glance.

### Event Correlation Engine
7 built-in patterns detect complex failure scenarios — from node cascade failures to OOM kill chains. Reduces alert noise by 10-50x through intelligent deduplication and cooldown.

### Single Binary, Zero Dependencies
CLI + Guardian daemon + REST API + Web Dashboard — all embedded in one binary. No database servers, no message queues, no external dependencies.

---

## Features

### CLI Commands

| Command | Description |
|---|---|
| `otacon scan` | Full cluster health scan with A-F scorecard |
| `otacon scan --explain` | Detailed findings with remediation guidance |
| `otacon scan --export report.html` | Professional HTML report export |
| `otacon scan --exclude-categories "Network Policies"` | Exclude specific categories |
| `otacon scan --severity critical` | Filter findings by severity |
| `otacon scan --min-score 70` | CI/CD quality gate (exit 1 if below threshold) |
| `otacon audit` | Best practice compliance check |
| `otacon diagnose network\|logs\|nodes\|all` | Deep diagnostics |
| `otacon resources` | Missing requests/limits/quotas analysis |
| `otacon events --since 1h --correlate` | Correlated event timeline |
| `otacon guardian start --port 8080 --ui` | Continuous monitoring with web dashboard |
| `otacon completion bash\|zsh\|fish` | Shell completions |

### Audit Engine — 50+ Rules

| Category | Rules | Key Checks |
|---|---|---|
| Security | 11 | Root containers, privileged mode, host networking, capabilities, service account tokens |
| Resource Management | 8 | CPU/memory requests & limits, ResourceQuotas, LimitRanges, HPA |
| Reliability | 7 | Liveness/readiness/startup probes, replica count, PDB, anti-affinity |
| Best Practices | 5 | Image tags, labels, trusted registries, deployment strategy |
| Network Policies | 3 | Policy existence, default deny, service exposure |

### Intelligence Engine

| Pattern | Detection |
|---|---|
| Node Cascade Failure | Node down → evictions → scheduling failures |
| OOM Cascade | Memory kill → crash loop → restart storm |
| Image Pull Chain | Registry failure → blocked deployments |
| Volume Mount Failure | PVC issues → stuck pods |
| Scheduling Pressure | Resource exhaustion across cluster |
| DNS Resolution Failure | CoreDNS issues → service breakdown |
| Probe Failure Cascade | Health check failures → restart loops |

Additional intelligence features:
- **Smart Deduplication** — 50 identical events → 1 summary with occurrence count
- **Severity-Based Cooldown** — Critical: 15min, Warning: 60min, Info: 6h
- **Safety Valve** — Forces notification after 100 suppressed events
- **Daily/Weekly Digests** — Scheduled summary reports

### Notification Channels

| Channel | Format | Features |
|---|---|---|
| Slack | Block Kit | Color-coded severity, field sections, enrichment blocks |
| Microsoft Teams | MessageCard | Adaptive cards with action buttons |
| Email | HTML | Formatted reports via SMTP |
| Webhook | JSON | Generic HTTP POST for custom integrations |

Enrichment plugins attach context to every notification:
- **Pod Logs** — Recent log lines from affected containers
- **Grafana Links** — Direct links to relevant dashboards
- **Runbook URLs** — Operational playbook references

### Web Dashboard

5-page dark-themed UI embedded in the binary:

- **Dashboard** — Score trend chart, critical event counter, incident timeline
- **Events** — Real-time event stream with severity and namespace filters
- **Audit** — Interactive scorecard with expandable category findings
- **Resources** — Missing limits/requests analysis per namespace
- **Status** — System health, storage stats, notification metrics

### Guardian Mode

Long-running daemon for continuous cluster monitoring:

- Real-time Kubernetes event watching via Watch API
- Automatic event correlation and deduplication
- Periodic audit scans (every 6 hours)
- REST API on configurable port
- Prometheus `/metrics` endpoint (10 metrics)
- Runs locally or in-cluster as a Deployment

### Kubernetes Integration

- **Helm Chart** — Full deployment with RBAC, PVC, Ingress, ServiceMonitor
- **Kustomize** — Base overlay for GitOps workflows
- **Standalone Manifests** — Plain YAML for quick deployment
- **3 CRDs** — `NotificationRule`, `AuditPolicy`, `DigestSchedule`
- **In-cluster RBAC** — Read-only ClusterRole with minimal permissions

### Observability

10 Prometheus metrics:

```
otacon_events_total
otacon_events_by_severity{severity="critical|warning|info"}
otacon_incidents_total
otacon_notifications_sent_total
otacon_notifications_suppressed_total
otacon_notifications_errors_total
otacon_audit_score
otacon_audit_grade{grade="A+|A|B+|..."}
otacon_uptime_seconds
otacon_info{version="0.1.0"}
```

---

## Installation

```bash
# Quick install script
curl -sSL https://raw.githubusercontent.com/merthan/otacon/main/scripts/install.sh | sh

# Homebrew (macOS/Linux)
brew install merthan/tap/otacon

# Go install
go install github.com/merthan/otacon/cmd/otacon@v0.1.0

# Docker
docker pull ghcr.io/merthan/otacon:v0.1.0

# Helm (in-cluster)
helm install otacon ./deploy/helm/otacon --namespace otacon-system --create-namespace
```

---

## Quick Start

```bash
# Scan your cluster
otacon scan

# Detailed audit with fix suggestions
otacon scan --explain

# Export professional report
otacon scan --export report.html

# Deep diagnostics
otacon diagnose all

# Start monitoring dashboard
otacon guardian start --port 8080 --ui
# Open http://localhost:8080
```

---

## Technical Details

- **Language:** Go 1.22+
- **Storage:** SQLite (pure Go driver, WAL mode, 7-day retention)
- **Binary size:** ~25MB (includes embedded Web UI)
- **Architecture:** Single binary, event-driven, read-only cluster access
- **Platforms:** Linux (amd64/arm64), macOS (Intel/Apple Silicon), Windows

---

## What's Next (Roadmap)

- [ ] UI-based notification channel management
- [ ] Alertmanager webhook bridge
- [ ] PagerDuty / OpsGenie integration
- [ ] Custom rule engine (YAML-defined audit rules)
- [ ] Multi-cluster federation
- [ ] RBAC-aware dashboard

---

**Full Changelog:** https://github.com/merthan/otacon/commits/v0.1.0
