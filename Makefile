BINARY        := tsc
PKG           := ./cmd/tsc
DIST          := dist
GOLANGCI_LINT ?= golangci-lint
LINT_CONFIG   := .github/.golangci.yml

# Platforms built by build-all, as GOOS/GOARCH.
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

GOFLAGS_BUILD := -trimpath -ldflags="-s -w"

.DEFAULT_GOAL := build

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build tsc for the current platform
	CGO_ENABLED=0 go build $(GOFLAGS_BUILD) -o $(BINARY) $(PKG)

.PHONY: build-all
build-all: $(PLATFORMS) ## Build tsc for every platform into dist/

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	$(eval GOOS := $(word 1,$(subst /, ,$@)))
	$(eval GOARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GOFLAGS_BUILD) \
		-o $(DIST)/$(BINARY)_$(GOOS)_$(GOARCH)$(EXT) $(PKG)

.PHONY: test
test: ## Run the tests
	go test -race -cover ./...

.PHONY: lint
lint: ## Run golangci-lint
	@command -v $(GOLANGCI_LINT) >/dev/null || { \
		echo "$(GOLANGCI_LINT) not found, see https://golangci-lint.run/welcome/install/"; exit 1; }
	$(GOLANGCI_LINT) run --config $(LINT_CONFIG) ./...

.PHONY: fmt
fmt: ## Format the code with golangci-lint formatters
	$(GOLANGCI_LINT) fmt --config $(LINT_CONFIG) ./...

.PHONY: check
check: lint test ## Run lint and tests

.PHONY: clean
clean: ## Remove build outputs
	rm -rf $(BINARY) $(DIST)
