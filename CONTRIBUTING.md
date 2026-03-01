# Contributing to Otacon

Thank you for your interest in contributing to Otacon! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.22+
- A Kubernetes cluster (kind, minikube, or real cluster)
- kubectl configured

### Build from Source

```bash
git clone https://github.com/merthan/otacon.git
cd otacon
make build
./bin/otacon version
```

### Running Tests

```bash
# Unit tests
make test

# Lint
make lint

# Format code
make fmt
```

## How to Contribute

### Reporting Bugs

Open an issue using the **Bug Report** template. Include:
- Otacon version (`otacon version`)
- Kubernetes version (`kubectl version`)
- Steps to reproduce
- Expected vs actual behavior

### Suggesting Features

Open an issue using the **Feature Request** template. Describe:
- The problem you're solving
- Your proposed solution
- Alternative approaches considered

### Submitting Code

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes
4. Write tests for new functionality
5. Ensure all tests pass: `make test`
6. Commit with conventional commits: `git commit -m "feat: add network policy scoring"`
7. Push and open a Pull Request

### Adding New Audit Rules

Audit rules live in `internal/engine/audit/rules.go`. To add a new rule:

1. Define the rule in the appropriate category function (e.g., `securityRules()`)
2. Implement the check function
3. Include ID, name, category, severity, description, and explain text
4. Add tests in `internal/engine/audit/rules_test.go`

### Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — New feature
- `fix:` — Bug fix
- `docs:` — Documentation
- `test:` — Tests
- `refactor:` — Code refactoring
- `ci:` — CI/CD changes
- `chore:` — Maintenance

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
