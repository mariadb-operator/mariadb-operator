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

VERSION ?= 0.39.0-dev

# mariadb-operator
IMG_NAME ?= docker-registry3.mariadb.com/mariadb-operator/mariadb-operator
IMG ?= $(IMG_NAME):$(VERSION)

# mariadb
RELATED_IMAGE_MARIADB_NAME ?= docker-registry1.mariadb.com/library/mariadb
RELATED_IMAGE_MARIADB_VERSION ?= 11.8.2
# RELATED_IMAGE_MARIADB_VERSION ?= 11.8.2-ubi
RELATED_IMAGE_MARIADB ?= $(RELATED_IMAGE_MARIADB_NAME):$(RELATED_IMAGE_MARIADB_VERSION)
MARIADB_DEFAULT_VERSION ?= 11.8
MARIADB_GALERA_LIB_PATH ?= /usr/lib/galera/libgalera_smm.so
# MARIADB_GALERA_LIB_PATH ?= /usr/lib64/galera/libgalera_smm.so

# maxscale
RELATED_IMAGE_MAXSCALE_NAME ?= docker-registry2.mariadb.com/mariadb/maxscale
RELATED_IMAGE_MAXSCALE_VERSION ?= 23.08.5
RELATED_IMAGE_MAXSCALE ?= $(RELATED_IMAGE_MAXSCALE_NAME):$(RELATED_IMAGE_MAXSCALE_VERSION)

# mysqld-exporter
RELATED_IMAGE_EXPORTER_NAME ?= prom/mysqld-exporter
RELATED_IMAGE_EXPORTER_VERSION ?= v0.15.1
RELATED_IMAGE_EXPORTER ?= $(RELATED_IMAGE_EXPORTER_NAME):$(RELATED_IMAGE_EXPORTER_VERSION)

# maxscale-exporter
RELATED_IMAGE_EXPORTER_MAXSCALE_NAME ?= docker-registry2.mariadb.com/mariadb/maxscale-prometheus-exporter-ubi
RELATED_IMAGE_EXPORTER_MAXSCALE_VERSION ?= v0.0.1
RELATED_IMAGE_EXPORTER_MAXSCALE ?= $(RELATED_IMAGE_EXPORTER_MAXSCALE_NAME):$(RELATED_IMAGE_EXPORTER_MAXSCALE_VERSION)

# docker
MARIADB_DOCKER_REPO ?= https://github.com/MariaDB/mariadb-docker
MARIADB_DOCKER_COMMIT_HASH ?= eaf65f505334595d2eeb21877daf6e43f46d8b3b
MARIADB_ENTRYPOINT_PATH ?= pkg/embed/mariadb-docker
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
include make/docs.mk
include make/gen.mk
include make/helm.mk
include make/net.mk
include make/pki.mk
