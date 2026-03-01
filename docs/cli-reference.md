# CLI Reference

## Global Flags
| Flag | Short | Description |
|------|-------|-------------|
| `--kubeconfig` | | Path to kubeconfig |
| `--context` | | Kubernetes context |
| `--namespace` | `-n` | Target namespace (default: all) |
| `--output` | `-o` | Output format: table, json, yaml |
| `--verbose` | `-v` | Verbose output |
| `--no-color` | | Disable colors |

## Commands

### `otacon scan`
Full cluster health scan with A-F scorecard.
- `--explain` — Detailed explanations for findings
- `--categories` — Filter categories (comma-separated)
- `--export <file>` — Export report (.json, .html)
- `--top <n>` — Number of top findings (default 10)

### `otacon audit`
Best practice compliance check.
- `--severity <level>` — Filter: critical, warning, info
- `--explain` — Include remediation
- `--export <file>` — Export findings

### `otacon diagnose <target>`
Targets: `network`, `logs`, `nodes`, `all`

### `otacon resources`
Resource configuration analysis.

### `otacon events`
Event timeline with correlation.
- `--since <duration>` — Time window (default 1h)
- `--correlate` — Enable event correlation
- `--severity <level>` — Filter by severity

### `otacon guardian start`
Start guardian (daemon) mode.
- `--port <n>` — API port (default 8080)
- `--metrics-port <n>` — Metrics port (default 9090)
- `--ui` — Enable web UI (default true)

### `otacon version`
Print version information.
