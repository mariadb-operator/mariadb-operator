ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/../..

LOCALBIN ?= $(ROOT_DIR)/bin

include $(ROOT_DIR)/make/deps.mk
include $(ROOT_DIR)/make/deploy.mk
include $(ROOT_DIR)/make/pki.mk

OPERATOR_VERSION ?= 26.6.0
HELM_OCI_REGISTRY ?= oci://ghcr.io/mariadb-operator/charts
ENVOY_GATEWAY_VERSION ?= v1.8.1
KUBECONFIG_EU_SOUTH ?= /tmp/kubeconfig-eu-south
KUBECONFIG_EU_CENTRAL ?= /tmp/kubeconfig-eu-central
KUBECONFIG_ENVOY ?= /tmp/kubeconfig-envoy

##@ Install

.PHONY: multi-cluster
multi-cluster: host clusters pki config coredns operator metallb minio envoy-gateway ## Install the full multi-cluster demo environment.

##@ App

APP_FLAGS ?= --host=app.mariadb.com --port=3306 \
	--username=root --password=MariaDB11! --database=test --table=test --timeout=3s \
	--ca-cert=/tmp/pki/ca/server.crt
.PHONY: app
app: ## Run the client app against app.mariadb.com in insert mode.
	$(GO) run $(ROOT_DIR)/hack/client/*.go $(APP_FLAGS) 

##@ Cluster

.PHONY: cluster-eu-south
cluster-eu-south: ## Create eu-south kind cluster.
	$(MAKE) cluster CLUSTER=eu-south

.PHONY: cluster-eu-central
cluster-eu-central: ## Create eu-central kind cluster.
	$(MAKE) cluster CLUSTER=eu-central

.PHONY: cluster-envoy
cluster-envoy: ## Create envoy kind cluster.
	$(MAKE) cluster CLUSTER=envoy

.PHONY: clusters
clusters: cluster-eu-south cluster-eu-central cluster-envoy ## Create all kind clusters.

.PHONY: cluster-delete-eu-south
cluster-delete-eu-south: ## Delete eu-south kind cluster.
	$(MAKE) cluster-delete CLUSTER=eu-south

.PHONY: cluster-delete-eu-central
cluster-delete-eu-central: ## Delete eu-central kind cluster.
	$(MAKE) cluster-delete CLUSTER=eu-central

.PHONY: cluster-delete-envoy
cluster-delete-envoy: ## Delete envoy kind cluster.
	$(MAKE) cluster-delete CLUSTER=envoy

.PHONY: clusters-delete
clusters-delete: cluster-delete-eu-south cluster-delete-eu-central cluster-delete-envoy ## Delete all kind clusters.

##@ Config

.PHONY: kubeconfig
kubeconfig: kind ## Export kubeconfig for all clusters.
	$(KIND) export kubeconfig --name eu-south --kubeconfig $(KUBECONFIG_EU_SOUTH)
	$(KIND) export kubeconfig --name eu-central --kubeconfig $(KUBECONFIG_EU_CENTRAL)
	$(KIND) export kubeconfig --name envoy --kubeconfig $(KUBECONFIG_ENVOY)

.PHONY: pki
pki: kubeconfig ## Generate PKI and install the same certificates in both clusters.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) $(MAKE) cert
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) $(MAKE) cert

.PHONY: config-eu-south
config-eu-south: kubeconfig ## Install common configuration in eu-south.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) kubectl apply -f $(ROOT_DIR)/examples/manifests/config

.PHONY: config-eu-central
config-eu-central: kubeconfig ## Install common configuration in eu-central.
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) kubectl apply -f $(ROOT_DIR)/examples/manifests/config

.PHONY: config
config: config-eu-south config-eu-central ## Install common configuration in all clusters.

.PHONY: coredns-eu-south
coredns-eu-south: kubeconfig ## Patch CoreDNS ConfigMap and restart CoreDNS in eu-south.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) $(CURDIR)/hack/patch_coredns.sh

.PHONY: coredns-eu-central
coredns-eu-central: kubeconfig ## Patch CoreDNS ConfigMap and restart CoreDNS in eu-central.
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) $(CURDIR)/hack/patch_coredns.sh

.PHONY: coredns-envoy
coredns-envoy: kubeconfig ## Patch CoreDNS ConfigMap and restart CoreDNS in envoy.
	KUBECONFIG=$(KUBECONFIG_ENVOY) $(CURDIR)/hack/patch_coredns.sh

.PHONY: coredns
coredns: coredns-eu-south coredns-eu-central coredns-envoy ## Patch CoreDNS in all clusters.

.PHONY: host
host: ## Add app.mariadb.com to /etc/hosts.
	@$(ROOT_DIR)/hack/add_host.sh 1 10 mariadb-eu-south.mariadb.com
	@$(ROOT_DIR)/hack/add_host.sh 1 15 mariadb-eu-central.mariadb.com
	@$(ROOT_DIR)/hack/add_host.sh 1 100 app.mariadb.com

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

.PHONY: metallb-eu-south
metallb-eu-south: kubeconfig helm ## Install metallb in eu-south.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) METALLB_VERSION=$(METALLB_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_metallb.sh

.PHONY: metallb-eu-central
metallb-eu-central: kubeconfig helm ## Install metallb in eu-central.
	KUBECONFIG=$(KUBECONFIG_EU_CENTRAL) METALLB_VERSION=$(METALLB_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_metallb.sh

.PHONY: metallb-envoy
metallb-envoy: kubeconfig helm ## Install metallb in envoy cluster.
	KUBECONFIG=$(KUBECONFIG_ENVOY) METALLB_VERSION=$(METALLB_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_metallb.sh

.PHONY: metallb
metallb: metallb-eu-south metallb-eu-central metallb-envoy ## Install metallb in all clusters.

.PHONY: envoy-gateway
envoy-gateway: helm kubeconfig ## Install Envoy Gateway in the envoy cluster.
	KUBECONFIG=$(KUBECONFIG_ENVOY) ENVOY_GATEWAY_VERSION=$(ENVOY_GATEWAY_VERSION) HELM=$(HELM) $(CURDIR)/hack/install_envoy_gateway.sh

.PHONY: minio
minio: helm ## Install minio in eu-south.
	KUBECONFIG=$(KUBECONFIG_EU_SOUTH) MINIO_VERSION=$(MINIO_VERSION) HELM=$(HELM) $(ROOT_DIR)/hack/install_minio.sh