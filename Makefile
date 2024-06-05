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

VERSION ?= 0.0.29

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

RELATED_IMAGE_MARIADB_ENT_NAME ?= mariadbmmontes/enterprise-server
# RELATED_IMAGE_MARIADB_ENT_NAME ?= docker-registry.mariadb.com/enterprise-server
RELATED_IMAGE_MARIADB_ENT_VERSION ?= 10.6
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
MARIADB_GALERA_INIT_IMAGE ?= $(IMG)
MARIADB_GALERA_INIT_IMAGE_ENT ?= $(IMG_ENT)
MARIADB_GALERA_AGENT_IMAGE ?= $(IMG)
MARIADB_GALERA_AGENT_IMAGE_ENT ?= $(IMG_ENT)
MARIADB_GALERA_LIB_PATH ?= /usr/lib/galera/libgalera_smm.so
MARIADB_GALERA_LIB_PATH_ENT ?= /usr/lib64/galera/libgalera_smm.so

# docker
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
include make/openshift.mk
include make/pki.mk