BINARY := mkgo
PKG := ./...
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PREFIX ?= $(HOME)/.local

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build bin/mkgo
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

.PHONY: install
install: ## Install mkgo into $(PREFIX)/bin
	go build -ldflags "$(LDFLAGS)" -o $(PREFIX)/bin/$(BINARY) .

.PHONY: test
test: ## Run the tests
	go test -race $(PKG)

.PHONY: cover
cover: ## Run the tests and open the coverage report
	go test -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out

.PHONY: fmt
fmt: ## Format the code
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	go vet $(PKG)

.PHONY: check
check: fmt vet test ## Format, vet and test

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin coverage.out
