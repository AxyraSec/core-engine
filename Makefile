BINARY     := engine
BIN_DIR    := bin
PKG        := ./cmd/engine
IMAGE      ?= ghcr.io/axyrasec/core-engine
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DOCKERFILE := deploy/docker/Dockerfile

LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

.PHONY: lint
lint: ## Run golangci-lint
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not found: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

.PHONY: test
test: ## Run unit tests with the race detector
	go test -race -count=1 ./...

.PHONY: build
build: ## Build a static binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(PKG)

.PHONY: dist
dist: ## Build release binaries for linux, macOS and windows into dist/
	scripts/build-release.sh $(VERSION)

.PHONY: docker
docker: ## Build the container image (no push)
	docker build -f $(DOCKERFILE) --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

.PHONY: security
security: ## Run govulncheck and gitleaks
	@command -v govulncheck >/dev/null || { echo "govulncheck not found: go install golang.org/x/vuln/cmd/govulncheck@v1.8.0"; exit 1; }
	@command -v gitleaks >/dev/null || { echo "gitleaks not found: https://github.com/gitleaks/gitleaks#installing"; exit 1; }
	govulncheck ./...
	gitleaks git --redact .

.PHONY: demo
demo: build ## Run the MCP proxy demo (allow, deny, evidence, tamper detection)
	go build -o $(BIN_DIR)/mcp-demo-server ./test/mcp-demo-server
	go run ./test/demo -engine $(BIN_DIR)/$(BINARY) -server $(BIN_DIR)/mcp-demo-server -policy policies/example.yaml

.PHONY: docker-demo
docker-demo: ## Build the demo image (no push)
	docker build -f deploy/docker/Dockerfile.demo --build-arg VERSION=$(VERSION) -t $(IMAGE)-demo:$(VERSION) .

.PHONY: versions
versions: ## Update the versions table in SECURITY.md
	go run ./scripts/versions

.PHONY: clean
clean: ## Remove build output
	rm -rf $(BIN_DIR)
