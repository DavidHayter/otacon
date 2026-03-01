# ADR-002: Rule-Based Event Correlation

## Status
Accepted

## Context
Events in Kubernetes fire independently. A node failure generates 10-50+ separate events. We need a way to correlate these into meaningful incidents.

## Decision
Rule-based correlation with configurable time windows, node/namespace constraints, and built-in patterns for common failure cascades.

## Rationale
- **Predictable**: Rules are explicit, auditable, and debuggable
- **Extensible**: New rules can be added via CRDs
- **Low latency**: Pattern matching on buffered events is fast
- **No training data needed**: Works from day one, no ML model to bootstrap

## Trade-offs
- Cannot detect novel correlation patterns (requires ML)
- Rules need maintenance as K8s evolves
- False positives possible with wide time windows

## Built-in Rules (v0.1)
1. Node Cascade Failure (10min window, same node)
2. OOM Cascade (15min, same namespace)
3. Image Pull Chain (10min, same namespace)
4. Volume Mount Chain (10min, same namespace)
5. Scheduling Pressure (5min, cross-namespace)
6. DNS Resolution Failure (5min, same namespace)
7. Probe Failure Cascade (10min, same namespace)

## Future
Consider optional ML-based anomaly detection as a plugin for v2.0.
