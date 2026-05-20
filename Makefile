# Development commands. Run `make help` for a list.
# CI uses the inline commands in .github/workflows/ci.yml so this Makefile
# stays out of the critical path — it's a developer convenience only.

GO        ?= go
GOFLAGS   ?=
COVERAGE  ?= coverage.out

.DEFAULT_GOAL := help

.PHONY: help
help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Compile all packages
	$(GO) build $(GOFLAGS) ./...

.PHONY: test
test: ## Run tests (no race detector)
	$(GO) test $(GOFLAGS) ./...

.PHONY: test-race
test-race: ## Run tests with -race and coverage (matches CI)
	$(GO) test $(GOFLAGS) -race -coverprofile=$(COVERAGE) ./...

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## golangci-lint (requires local install)
	golangci-lint run

.PHONY: fmt
fmt: ## Format Go source with gofmt -s -w
	gofmt -s -w .

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: check
check: vet lint test-race ## Composite: vet + lint + test-race (run before pushing)

.PHONY: clean
clean: ## Remove coverage artifacts
	rm -f $(COVERAGE)
