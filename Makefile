SHELL := /bin/bash
NAME := $(shell echo $${PWD\#\#*/})
PKG := git.nygenome.org/rmusunuri/$(NAME)
VERSION := $(shell cat VERSION.txt)
BUILDTIME := $(shell date +'%Y-%m-%dT%H:%M:%S%Z')
GITCOMMIT := $(shell git rev-parse --short HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
GITCOMMIT := $(GITCOMMIT)-dirty
endif

GO := go
dep := $(GOBIN)/dep
packr := $(GOBIN)/packr
sembump := $(GOBIN)/sembump
git-chlog := $(GOBIN)/git-chlog
LDFLAGS=-ldflags "-w -X=$(PKG).Version=$(VERSION) -X=main.buildTime=$(BUILDTIME) -X=main.gitCommit=$(GITCOMMIT)"
SRC = $(shell find . -type f -not -path "./vendor/*" -and -name '*.go' -and -not -name '*.pb.go')

TARGET := $(NAME)
.DEFAULT_GOAL: $(TARGET)

.PHONY: all
all: install

$(TARGET): $(SRC)
	@$(GO) build $(LDFLAGS) cmd/*.go

$(packr): ## Install packr to embed resources
	@echo "+ $@"
	@$(GO) get github.com/gobuffalo/packr

$(dep): ## Install dep to install dependencies
	@echo "+ $@"
	@curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

$(sembump): ## Install sembump to automate semantic versioning
	@echo "+ $@"
	@$(GO) get github.com/jfinken/sembump

$(git-chlog): ## Install sembump to automate changelogs
	@echo "+ $@"
	@$(GO) get github.com/git-chglog/git-chglog/cmd/git-chglog

.PHONY: dep_ensure
dep_ensure: $(dep) ## Pin dependencies for builds
	@echo "+ $@"
	@$(dep) ensure

.PHONY: packr_gen
packr_gen: $(packr) ## Embed sites data into package for bundling
	@echo "+ $@"
	@$(packr)

.PHONY: packr_clean
packr_clean: $(packr) ## Remove and clean packr generated files
	@echo "+ $@"
	@$(packr) clean

.PHONY: build
build: lint dep_ensure packr_gen $(TARGET) ## Builds a snifty executable for current OS/Arch
	@echo "+ $@"
	@true
	@$(packr) clean

.PHONY: linux64
linux64: dep_ensure packr_gen ## Builds a snifty executable for linux/amd64 in bin
	@echo "+ $@"
	@GOOS=linux GOARCH=amd64 go build -o bin/$(TARGET)_linux64 $(LDFLAGS) cmd/*.go
	@$(packr) clean

.PHONY: osx64
osx64: dep_ensure packr_gen ## Builds a snifty executable for osx/amd64 in bin
	@echo "+ $@"
	@GOOS=darwin GOARCH=amd64 go build -o bin/$(TARGET)_osx64 $(LDFLAGS) cmd/*.go
	@$(packr) clean

.PHONY: win64
win64: dep_ensure packr_gen ## Builds a snifty executable for linux/amd64 in bin
	@echo "+ $@"
	@GOOS=windows GOARCH=amd64 go build -o bin/$(TARGET)_win64.exe $(LDFLAGS) cmd/*.go
	@$(packr) clean

.PHONY: deploy
deploy: clean lint dep_ensure packr_gen linux64 osx64 win64 packr_clean ### Builds and deploys linux64, osx64, win64 binaries to gitlab
	@echo "+ $@"
	@$(GO) build -o bin/deploybot tools/deploybot.go
	@bin/deploybot -version $(VERSION) -pid 214 bin/$(TARGET)_linux64 bin/$(TARGET)_osx64 bin/$(TARGET)_win64.exe
	@$(packr) clean

.PHONY: clean
clean: $(packr) ## Cleanup built and installed binaries
	@echo "+ $@"
	@rm -rf bin/
	@rm -f $(TARGET)
	@$(packr) clean

.PHONY: install
install: lint dep_ensure packr_gen ## Installs the snifty executable in $GOBIN
	@echo "+ $@"
	@$(GO) install $(LDFLAGS) cmd/*.go
	@$(packr) clean

bin/golangci-lint: ## Installs golangci-lint to run checks
	@echo "+ $@"
	@curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s v1.9.1

.PHONY: lint
lint: bin/golangci-lint ## Verifies that various golangci-lint checks passes
	@echo "+ $@"
	@bin/golangci-lint run -c .golangci-lint.yml

.PHONY: test
test: ### Runs the go unit tests
	@echo "+ $@"
	@$(GO)test -v $(shell $(GO) list ./... | grep -v vendor)

.PHONY: bump
BUMP := patch
bump: $(sembump) $(git-chlog) ### Bump version and tag new version. Set BUMP to [ patch | major | minor ]
	$(eval NEW_VERSION = $(shell sembump --kind $(BUMP) $(VERSION)))
	@echo "Bumping VERSION.txt from $(VERSION) to $(NEW_VERSION)"
	@echo $(NEW_VERSION) > VERSION.txt
	@git-chglog --next-tag $(NEW_VERSION) -o CHANGELOG.md
	git add VERSION.txt README.md CHANGELOG.md
	git commit -vsam "chore: Bump version to $(NEW_VERSION)"
	git tag -m "$(NAME) $(NEW_VERSION)" -sa $(NEW_VERSION)

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
