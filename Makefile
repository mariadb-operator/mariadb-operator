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

VERSION ?= 0.0.30-dev

# mariadb-operator
IMG_NAME ?= docker-registry3.mariadb.com/mariadb-operator/mariadb-operator
IMG_VERSION ?= v$(VERSION)
IMG ?= $(IMG_NAME):$(IMG_VERSION)

IMG_ENT_NAME ?= docker-registry2.mariadb.com/mariadb/mariadb-operator-enterprise
IMG_ENT_VERSION ?= v$(VERSION)
IMG_ENT ?= $(IMG_ENT_NAME):$(IMG_ENT_VERSION)

# mariadb
RELATED_IMAGE_MARIADB_NAME ?= docker-registry1.mariadb.com/library/mariadb
RELATED_IMAGE_MARIADB_VERSION ?= 10.11.8
RELATED_IMAGE_MARIADB ?= $(RELATED_IMAGE_MARIADB_NAME):$(RELATED_IMAGE_MARIADB_VERSION)

RELATED_IMAGE_MARIADB_ENT_NAME ?= docker-registry.mariadb.com/enterprise-server
RELATED_IMAGE_MARIADB_ENT_VERSION ?= 10.6.18-14
RELATED_IMAGE_MARIADB_ENT ?= $(RELATED_IMAGE_MARIADB_ENT_NAME):$(RELATED_IMAGE_MARIADB_ENT_VERSION)

# maxscale
RELATED_IMAGE_MAXSCALE_NAME ?= docker-registry2.mariadb.com/mariadb/maxscale
RELATED_IMAGE_MAXSCALE_VERSION ?= 23.08.5
RELATED_IMAGE_MAXSCALE ?= $(RELATED_IMAGE_MAXSCALE_NAME):$(RELATED_IMAGE_MAXSCALE_VERSION)

RELATED_IMAGE_MAXSCALE_ENT_NAME ?= docker-registry2.mariadb.com/mariadb/maxscale
RELATED_IMAGE_MAXSCALE_ENT_VERSION ?= 23.08.5-ubi
RELATED_IMAGE_MAXSCALE_ENT ?= $(RELATED_IMAGE_MAXSCALE_ENT_NAME):$(RELATED_IMAGE_MAXSCALE_ENT_VERSION)

# mysqld-exporter
RELATED_IMAGE_EXPORTER_NAME ?= prom/mysqld-exporter
RELATED_IMAGE_EXPORTER_VERSION ?= v0.15.1
RELATED_IMAGE_EXPORTER ?= $(RELATED_IMAGE_EXPORTER_NAME):$(RELATED_IMAGE_EXPORTER_VERSION)

RELATED_IMAGE_EXPORTER_ENT_NAME ?= docker-registry2.mariadb.com/mariadb/mariadb-prometheus-exporter-ubi
RELATED_IMAGE_EXPORTER_ENT_VERSION ?= v0.0.1
RELATED_IMAGE_EXPORTER_ENT ?= $(RELATED_IMAGE_EXPORTER_ENT_NAME):$(RELATED_IMAGE_EXPORTER_ENT_VERSION)

# maxscale-exporter
RELATED_IMAGE_EXPORTER_MAXSCALE_NAME ?= docker-registry2.mariadb.com/mariadb/maxscale-prometheus-exporter-ubi
RELATED_IMAGE_EXPORTER_MAXSCALE_VERSION ?= v0.0.1
RELATED_IMAGE_EXPORTER_MAXSCALE ?= $(RELATED_IMAGE_EXPORTER_MAXSCALE_NAME):$(RELATED_IMAGE_EXPORTER_MAXSCALE_VERSION)

RELATED_IMAGE_EXPORTER_MAXSCALE_ENT_NAME ?= docker-registry2.mariadb.com/mariadb/maxscale-prometheus-exporter-ubi
RELATED_IMAGE_EXPORTER_MAXSCALE_ENT_VERSION ?= v0.0.1
RELATED_IMAGE_EXPORTER_MAXSCALE_ENT ?= $(RELATED_IMAGE_EXPORTER_MAXSCALE_ENT_NAME):$(RELATED_IMAGE_EXPORTER_MAXSCALE_ENT_VERSION)

# galera
MARIADB_GALERA_LIB_PATH ?= /usr/lib/galera/libgalera_smm.so
MARIADB_GALERA_LIB_PATH_ENT ?= /usr/lib64/galera/libgalera_smm.so

# docker
MARIADB_DOCKER_REPO ?= https://github.com/MariaDB/mariadb-docker
MARIADB_DOCKER_COMMIT_HASH ?= 11135d071fd1fe355b1f7fa99b9d3b4a59bb5225
MARIADB_ENTRYPOINT_PATH ?= pkg/embed/mariadb-docker
MARIADB_ENTRYPOINT_VERSION ?= 10.11

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
include make/openshift.mk
include make/pki.mk