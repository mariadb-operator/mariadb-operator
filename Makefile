# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: help

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: build
build: op-build ## Build all.

##@ Operator

.PHONY: op-run
op-run: manifests generate lint ## Run a controller from your host.
	go run ./cmd/operator/main.go

.PHONY: op-build
op-build: generate ## Build manager binary.
	go build -o bin/manager cmd/operator/main.go

# Image URL to use all building/pushing image targets
OP_IMG ?= mmontes11/mariadb-operator:latest

.PHONY: op-docker-build
op-docker-build: ## Build docker image with the manager.
	docker build -t ${OP_IMG} .

.PHONY: op-docker-push
op-docker-push: ## Push docker image with the manager.
	docker push ${OP_IMG}

include make/deploy.mk
include make/dev.mk
include make/networking.mk
include make/tooling.mk