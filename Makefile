BINARY_NAME := otacon
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build clean test lint fmt vet install help

all: lint test build ## Run lint, test, and build

build: ## Build the binary
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/otacon/
	@echo "Done: bin/$(BINARY_NAME)"

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64   ./cmd/otacon/
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64   ./cmd/otacon/
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64  ./cmd/otacon/
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64  ./cmd/otacon/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/otacon/

install: build ## Install to $GOPATH/bin
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	@go test -race -coverprofile=coverage.out ./...
	@echo "Coverage:"
	@go tool cover -func=coverage.out | tail -1

test-e2e: ## Run end-to-end tests (requires cluster)
	@echo "Running e2e tests..."
	@go test -tags=e2e -v ./test/e2e/...

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run ./... 2>/dev/null || echo "golangci-lint not installed, skipping"
	@go vet ./...

fmt: ## Format code
	@gofmt -s -w .
	@goimports -w . 2>/dev/null || true

vet: ## Run go vet
	@go vet ./...

clean: ## Clean build artifacts
	@rm -rf bin/ coverage.out

docker-build: ## Build Docker image
	docker build -t ghcr.io/merthan/otacon:$(VERSION) .
	docker tag ghcr.io/merthan/otacon:$(VERSION) ghcr.io/merthan/otacon:latest

docker-push: ## Push Docker image
	docker push ghcr.io/merthan/otacon:$(VERSION)
	docker push ghcr.io/merthan/otacon:latest

helm-lint: ## Lint Helm chart
	helm lint deploy/helm/otacon/

helm-template: ## Render Helm templates
	helm template otacon deploy/helm/otacon/

generate-crds: ## Generate CRD manifests
	@echo "Generating CRD manifests..."
	@go run ./scripts/generate-crds.go

run-scan: build ## Build and run scan
	./bin/$(BINARY_NAME) scan

run-audit: build ## Build and run audit
	./bin/$(BINARY_NAME) audit --explain

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
