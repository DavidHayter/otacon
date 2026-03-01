# Audit Rules Reference

## Security (11 rules)
| ID | Rule | Severity | Description |
|----|------|----------|-------------|
| SEC-001 | no-root-containers | Critical | Containers must not run as root |
| SEC-002 | no-privileged-containers | Critical | No privileged mode |
| SEC-003 | no-host-network | Warning | No host networking |
| SEC-004 | no-host-pid | Warning | No host PID namespace |
| SEC-005 | no-host-ipc | Warning | No host IPC namespace |
| SEC-006 | read-only-root-filesystem | Warning | Use read-only root FS |
| SEC-007 | no-capability-escalation | Critical | No privilege escalation |
| SEC-008 | drop-all-capabilities | Warning | Drop ALL capabilities |
| SEC-009 | no-default-service-account | Info | Don't use default SA |
| SEC-010 | no-automount-service-token | Warning | Don't auto-mount SA token |
| SEC-011 | image-pull-policy-always | Info | Use Always pull policy |

## Resource Management (8 rules)
| ID | Rule | Severity | Description |
|----|------|----------|-------------|
| RES-001 | cpu-requests-defined | Warning | CPU requests required |
| RES-002 | memory-requests-defined | Warning | Memory requests required |
| RES-003 | cpu-limits-defined | Warning | CPU limits required |
| RES-004 | memory-limits-defined | Critical | Memory limits required |
| RES-005 | resource-ratio-check | Info | Limit/request ratio <5x |
| RES-006 | namespace-resource-quota | Warning | ResourceQuota exists |
| RES-007 | namespace-limit-range | Info | LimitRange exists |
| RES-008 | hpa-configuration | Info | Consider HPA |

## Reliability (7 rules)
| ID | Rule | Severity | Description |
|----|------|----------|-------------|
| REL-001 | liveness-probe-defined | Warning | Liveness probe required |
| REL-002 | readiness-probe-defined | Warning | Readiness probe required |
| REL-003 | startup-probe-for-slow-start | Info | Startup probe for slow containers |
| REL-004 | single-replica-detection | Warning | >1 replica for production |
| REL-005 | pod-disruption-budget | Warning | PDB for multi-replica deploys |
| REL-006 | pod-anti-affinity | Info | Spread across nodes |
| REL-007 | restart-policy-check | Info | Restart policy matches workload |

## Best Practices (5 rules)
| ID | Rule | Severity | Description |
|----|------|----------|-------------|
| BP-001 | no-latest-tag | Warning | No :latest or untagged images |
| BP-002 | required-labels | Info | Standard labels present |
| BP-003 | container-image-registry | Info | Approved registries |
| BP-004 | deployment-strategy | Info | Update strategy configured |
| BP-005 | termination-grace-period | Info | Appropriate grace period |

## Network Policies (3 rules)
| ID | Rule | Severity | Description |
|----|------|----------|-------------|
| NET-001 | network-policy-exists | Critical | NetworkPolicy in namespace |
| NET-002 | default-deny-policy | Warning | Default deny policy |
| NET-003 | service-type-check | Warning | No unnecessary NodePort/LB |
