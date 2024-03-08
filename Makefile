# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifndef ignore-not-found
  ignore-not-found = false
endif

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

VERSION ?= 0.0.27

IMG_NAME ?= ghcr.io/mariadb-operator/mariadb-operator
IMG ?= $(IMG_NAME):v$(VERSION)
IMG_ENT_NAME ?= mariadb/mariadb-operator-enterprise
IMG_ENT ?= $(IMG_ENT_NAME):v$(VERSION)

RELATED_IMAGE_MARIADB ?= mariadb:11.2.2
RELATED_IMAGE_MARIADB_ENT ?= us-central1-docker.pkg.dev/mariadb-es-docker-registry/enterprise-docker/enterprise-server:10.6
# RELATED_IMAGE_MARIADB_ENT ?= docker.mariadb.com/enterprise-server:10.6

# TODO: certify image. UBI based and multi-arch.
RELATED_IMAGE_MAXSCALE ?= mariadb/maxscale:23.08
RELATED_IMAGE_EXPORTER ?= prom/mysqld-exporter:v0.15.1

MARIADB_GALERA_INIT_IMAGE ?= $(IMG_NAME):v0.0.27
MARIADB_GALERA_ENT_INIT_IMAGE ?= $(IMG_ENT_NAME):v0.0.27
MARIADB_GALERA_AGENT_IMAGE ?= $(IMG_NAME):v0.0.27
MARIADB_GALERA_ENT_AGENT_IMAGE ?= $(IMG_ENT_NAME):v0.0.27
MARIADB_GALERA_LIB_PATH ?= /usr/lib/galera/libgalera_smm.so
MARIADB_GALERA_ENT_LIB_PATH ?= /usr/lib/galera/libgalera_enterprise_smm.so

MARIADB_DOCKER_COMMIT_HASH ?= d7a950d41e9347ac94ad2d2f28469bff74858db7
MARIADB_DOCKER_URL ?= https://raw.githubusercontent.com/MariaDB/mariadb-docker/$(MARIADB_DOCKER_COMMIT_HASH)/10.11/docker-entrypoint.sh
MARIADB_DOCKER_PATH ?= pkg/embed/mariadb-docker/docker-entrypoint.sh

DOCKER_CONFIG ?= $(HOME)/.docker/config.json 

.PHONY: all
all: help

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: version
version: ## Get current version.
	@echo $(VERSION)

include make/build.mk
include make/deploy.mk
include make/deps.mk
include make/dev.mk
include make/gen.mk
include make/net.mk
include make/olm.mk
include make/pki.mk