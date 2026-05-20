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
sembump := sembump
git-chglog := git-chglog
TARGET := binest
GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

LDFLAGS := -ldflags "-w -X=$(PKG).Version=$(VERSION) -X=main.buildTime=$(BUILDTIME) -X=main.gitCommit=$(GITCOMMIT)"
SRC := $(shell find . -type f -not -path "./vendor/*" -and -name '*.go' -and -not -name '*.pb.go')

.DEFAULT_GOAL := build

.PHONY: all
all: test build

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

.PHONY: tidy
tidy: ## Tidy and verify module dependencies
	@echo "+ $@"
	@$(GO) mod tidy
	@$(GO) mod verify

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
lint: ## Run golangci-lint if installed
	@echo "+ $@"
	@golangci-lint run -c .golangci-lint.yml

.PHONY: bump
BUMP := patch
bump: ## Bump version and tag a release. Set BUMP to patch, minor, or major.
	$(eval NEW_VERSION = $(shell $(sembump) --kind $(BUMP) $(VERSION)))
	@echo "Bumping VERSION.txt from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION.txt
	@$(git-chglog) --next-tag $(NEW_VERSION) -o CHANGELOG.md
	git add VERSION.txt README.md CHANGELOG.md
	git commit -vsam "chore: Bump version to $(NEW_VERSION)"
	git tag -m "$(TARGET) $(NEW_VERSION)" -a $(NEW_VERSION)

.PHONY: dep_ensure
dep_ensure: tidy ## Alias for tidy, kept for compatibility

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
