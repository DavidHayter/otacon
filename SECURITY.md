# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Otacon, please report it responsibly.

**Do NOT open a public issue.**

Instead, please email: **security@otacon.dev** (or use GitHub Security Advisories)

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **24 hours**: Acknowledgment of your report
- **72 hours**: Initial assessment and severity classification
- **7 days**: Fix development and testing
- **14 days**: Release with fix (for critical issues, faster)

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | ✅        |
| < Latest | ❌       |

## Security Best Practices

When deploying Otacon in your cluster:

1. Use the provided `nonroot` container image
2. Apply the principle of least privilege via the provided ClusterRole
3. Restrict Otacon's namespace access via RBAC if you don't need cluster-wide scanning
4. Store notification webhook URLs in Kubernetes Secrets
5. Enable TLS for the web UI in production
