ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/../..

LOCALBIN ?= $(ROOT_DIR)/bin

include $(ROOT_DIR)/make/deps.mk
include $(ROOT_DIR)/make/deploy.mk
include $(ROOT_DIR)/make/pki.mk

OPERATOR_VERSION ?= 26.6.0
HELM_OCI_REGISTRY ?= oci://ghcr.io/mariadb-operator/charts
KUBECONFIG_EU_SOUTH ?= /tmp/kubeconfig-eu-south
KUBECONFIG_EU_CENTRAL ?= /tmp/kubeconfig-eu-central

##@ Cluster

.PHONY: cluster-eu-south
cluster-eu-south: ## Create eu-south kind cluster.
	$(MAKE) cluster CLUSTER=eu-south

.PHONY: cluster-eu-central
cluster-eu-central: ## Create eu-central kind cluster.
	$(MAKE) cluster CLUSTER=eu-central

.PHONY: clusters
clusters: cluster-eu-south cluster-eu-central ## Create all kind clusters.

.PHONY: cluster-delete-eu-south
cluster-delete-eu-south: ## Delete eu-south kind cluster.
	$(MAKE) cluster-delete CLUSTER=eu-south

.PHONY: cluster-delete-eu-central
cluster-delete-eu-central: ## Delete eu-central kind cluster.
	$(MAKE) cluster-delete CLUSTER=eu-central

.PHONY: clusters-delete
clusters-delete: cluster-delete-eu-south cluster-delete-eu-central ## Delete all kind clusters.

##@ Kubeconfig

.PHONY: kubeconfig
kubeconfig: kind ## Export kubeconfig for all clusters.
	$(KIND) export kubeconfig --name eu-south --kubeconfig $(KUBECONFIG_EU_SOUTH)
	$(KIND) export kubeconfig --name eu-central --kubeconfig $(KUBECONFIG_EU_CENTRAL)

##@ PKI

.PHONY: pki
pki: kubeconfig ## Generate PKI and install the same certificates in both clusters.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) $(MAKE) cert
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) $(MAKE) cert

##@ Deploy

.PHONY: operator-eu-south
operator-eu-south: helm kubeconfig ## Deploy the operator in eu-south via OCI Helm charts.
	$(HELM) install mariadb-operator-crds $(HELM_OCI_REGISTRY)/mariadb-operator-crds \
		--kubeconfig $(KUBECONFIG_EU_SOUTH) --version $(OPERATOR_VERSION)
	$(HELM) install mariadb-operator $(HELM_OCI_REGISTRY)/mariadb-operator \
		--kubeconfig $(KUBECONFIG_EU_SOUTH) --version $(OPERATOR_VERSION)

.PHONY: operator-eu-central
operator-eu-central: helm kubeconfig ## Deploy the operator in eu-central via OCI Helm charts.
	$(HELM) install mariadb-operator-crds $(HELM_OCI_REGISTRY)/mariadb-operator-crds \
		--kubeconfig $(KUBECONFIG_EU_CENTRAL) --version $(OPERATOR_VERSION)
	$(HELM) install mariadb-operator $(HELM_OCI_REGISTRY)/mariadb-operator \
		--kubeconfig $(KUBECONFIG_EU_CENTRAL) --version $(OPERATOR_VERSION)

.PHONY: operator
operator: operator-eu-south operator-eu-central ## Deploy the operator in all clusters.

##@ Install

.PHONY: metallb-eu-south
metallb-eu-south: kubeconfig helm ## Install metallb in eu-south.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) METALLB_VERSION=$(METALLB_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_metallb.sh

.PHONY: metallb-eu-central
metallb-eu-central: kubeconfig helm ## Install metallb in eu-central.
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) METALLB_VERSION=$(METALLB_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_metallb.sh

.PHONY: metallb
metallb: metallb-eu-south metallb-eu-central ## Install metallb in all clusters.

.PHONY: minio-eu-south
minio-eu-south: helm ## Install minio in eu-south.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) MINIO_VERSION=$(MINIO_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_minio.sh

.PHONY: minio-eu-central
minio-eu-central: helm ## Install minio in eu-central.
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) MINIO_VERSION=$(MINIO_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_minio.sh

.PHONY: minio
minio: minio-eu-south minio-eu-central ## Install minio in all clusters.
