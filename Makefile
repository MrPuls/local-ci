# local-ci developer Makefile.
#
# The web UI (web/, Bun + Vue 3) is built into internal/web/dist and embedded
# into the binary via //go:embed, so `local-ci ui` serves the whole app from a
# single executable. The built dist is committed, so a plain `go build` /
# `go install` already includes the UI — run `make web` and commit the result
# whenever you change the frontend.

BIN      := bin/local-ci
PKG      := ./cmd/local-ci
WEB_DIST := internal/web/dist

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: web
web: ## Build the web UI into internal/web/dist (commit the result)
	cd web && bun install && bun run build

.PHONY: build
build: web ## Build the local-ci binary with a freshly-built embedded UI
	go build -o $(BIN) $(PKG)

.PHONY: build-go
build-go: ## Build the binary only, using the already-committed UI build
	go build -o $(BIN) $(PKG)

.PHONY: install
install: web ## go install local-ci with a freshly-built embedded UI
	go install $(PKG)

.PHONY: ui
ui: build ## Build, then run `local-ci ui` (point it at a project with `cd`)
	./$(BIN) ui

.PHONY: dev-web
dev-web: ## Run the Vite dev server (needs `local-ci serve` running — see web/README.md)
	cd web && bun run dev

.PHONY: typecheck
typecheck: ## Type-check the web UI (vue-tsc)
	cd web && bun run typecheck

.PHONY: test
test: ## Run the Go test suite
	go test ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format Go sources
	gofmt -w .

.PHONY: clean
clean: ## Remove build output (bin/, goreleaser dist/)
	rm -rf bin dist
