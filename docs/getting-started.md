# Getting Started with Otacon

## Installation

```bash
# Homebrew
brew install merthan/tap/otacon

# Binary
curl -sSL https://github.com/merthan/otacon/releases/latest/download/otacon_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz | tar xz
sudo mv otacon /usr/local/bin/

# Go
go install github.com/merthan/otacon/cmd/otacon@latest
```

## Quick Start

```bash
# Verify installation
otacon version

# Full cluster scan
otacon scan

# Scan specific namespace
otacon scan -n production

# Detailed audit with remediation
otacon audit --explain

# Diagnostics
otacon diagnose network
otacon diagnose logs
otacon diagnose nodes

# Resource analysis
otacon resources -n production

# Event timeline
otacon events --since 1h --correlate
```

## Configuration

Otacon uses your standard kubeconfig:
```bash
otacon scan --kubeconfig ~/.kube/config --context production-cluster
```

## Guardian Mode (In-Cluster)

```bash
helm install otacon oci://ghcr.io/merthan/otacon/chart \
  --namespace otacon-system --create-namespace \
  --set notifications.slack.webhook=$SLACK_WEBHOOK
```
