SHELL := /bin/bash
PKG := git.nygenome.org/rmusunuri/binest
VERSION := $(shell cat VERSION.txt)
BUILDTIME := $(shell date +'%Y-%m-%dT%H:%M:%S%Z')
GITCOMMIT := $(shell git rev-parse --short HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
GITCOMMIT := $(GITCOMMIT)-dirty
endif

GO := go
GOLANGCI_LINT_VERSION := v2.12.2
TARGET := binest
GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

LDFLAGS := -ldflags "-w -X=$(PKG).Version=$(VERSION) -X=main.buildTime=$(BUILDTIME) -X=main.gitCommit=$(GITCOMMIT)"
SRC := $(shell find . -type f -not -path "./vendor/*" -and -name '*.go' -and -not -name '*.pb.go')
GOFILES := $(shell find . -type f -not -path "./vendor/*" -and -name '*.go')

.DEFAULT_GOAL := build

.PHONY: all
all: check

bin/$(TARGET): $(SRC) VERSION.txt
	@echo "+ $@"
	@mkdir -p bin
	@CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(TARGET) ./cmd/binest.go

.PHONY: build
build: bin/$(TARGET) ## Build the binest executable in bin/
	@echo "+ $@"

.PHONY: install
install: bin/$(TARGET) ## Install the binest executable in GOBIN or GOPATH/bin
	@echo "+ $@"
	@mkdir -p "$(GOBIN)"
	@cp bin/$(TARGET) "$(GOBIN)/$(TARGET)"

.PHONY: test
test: ## Run Go tests
	@echo "+ $@"
	@$(GO) test ./...

.PHONY: vet
vet: ## Run go vet
	@echo "+ $@"
	@$(GO) vet ./...

.PHONY: fmt
fmt: ## Format Go source files
	@echo "+ $@"
	@$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Verify Go source files are gofmt formatted
	@echo "+ $@"
	@test -z "$$(gofmt -l $(GOFILES))"

.PHONY: tidy
tidy: ## Tidy and verify module dependencies
	@echo "+ $@"
	@$(GO) mod tidy
	@$(GO) mod verify

.PHONY: tidy-check
tidy-check: ## Verify go.mod and go.sum are tidy
	@echo "+ $@"
	@$(GO) mod tidy -diff

.PHONY: linux64
linux64: ## Build the binest executable for linux/amd64 in bin/
	@echo "+ $@"
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(TARGET)_linux64 ./cmd/binest.go

.PHONY: osx64
osx64: ## Build the binest executable for darwin/amd64 in bin/
	@echo "+ $@"
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(TARGET)_osx64 ./cmd/binest.go

.PHONY: win64
win64: ## Build the binest executable for windows/amd64 in bin/
	@echo "+ $@"
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(TARGET)_win64.exe ./cmd/binest.go

.PHONY: clean
clean: ## Remove built binaries
	@echo "+ $@"
	@rm -rf bin/

.PHONY: lint
lint: ## Run pinned golangci-lint
	@echo "+ $@"
	@$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run -c .golangci-lint.yml

.PHONY: vuln
vuln: ## Run govulncheck
	@echo "+ $@"
	@$(GO) tool govulncheck ./...

.PHONY: check
check: fmt-check tidy-check vet test lint vuln build ## Run all local checks

.PHONY: dep_ensure
dep_ensure: tidy ## Alias for tidy, kept for compatibility

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
