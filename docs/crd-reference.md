# CRD Reference

Otacon uses Custom Resource Definitions for GitOps-native configuration in Guardian mode.

## NotificationRule

Controls when and where notifications are sent.

```yaml
apiVersion: otacon.io/v1alpha1
kind: NotificationRule
metadata:
  name: production-critical
  namespace: otacon-system
spec:
  match:
    namespaces: ["production", "payment-*"]
    severities: ["Critical"]
    eventReasons: ["OOMKilled", "CrashLoopBackOff", "NodeNotReady"]
  routing:
    - channel: slack
      target: "#prod-incidents"
      enrichment:
        - type: logs
          tailLines: 20
        - type: grafana
          dashboardUID: "pod-metrics"
          baseURL: "https://grafana.internal"
        - type: runbook
          baseURL: "https://wiki.internal"
    - channel: email
      target: "oncall@company.com"
  deduplication:
    windowMinutes: 15
  cooldown:
    durationMinutes: 60
```

### Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| `match.namespaces` | `[]string` | Namespace filter (supports `*` wildcards) |
| `match.severities` | `[]string` | Severity filter: Critical, Warning, Info |
| `match.eventReasons` | `[]string` | K8s event reason filter |
| `routing[].channel` | `string` | Channel type: slack, teams, email, webhook |
| `routing[].target` | `string` | Channel-specific target |
| `routing[].enrichment` | `[]object` | Enrichment plugins to apply |
| `deduplication.windowMinutes` | `int` | Dedup window (default: 15) |
| `cooldown.durationMinutes` | `int` | Cooldown period (default: 60) |

## AuditPolicy

Defines custom audit rules beyond the built-in 50+.

```yaml
apiVersion: otacon.io/v1alpha1
kind: AuditPolicy
metadata:
  name: custom-label-check
spec:
  rules:
    - id: "CUSTOM-001"
      name: "require-cost-center-label"
      category: "Best Practices"
      severity: Warning
      description: "All deployments must have a cost-center label"
      checkType: labels
      match:
        kind: Deployment
        requiredLabels: ["cost-center", "team", "service"]
      remediation: "Add cost-center label to your Deployment metadata"
```

## DigestSchedule

Configures periodic cluster health digests.

```yaml
apiVersion: otacon.io/v1alpha1
kind: DigestSchedule
metadata:
  name: daily-summary
spec:
  schedule: "0 9 * * *"       # 9 AM daily
  type: comprehensive          # comprehensive | events-only | audit-only
  channels: ["slack", "email"]
  includeScorecard: true
  includeTrending: true
  topEvents: 10
```

### Schedule Types

| Type | Content |
|------|---------|
| `comprehensive` | Scorecard + events + incidents + trending |
| `events-only` | Event summary and top event groups |
| `audit-only` | Scorecard and findings |
