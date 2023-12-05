HELM_DIR ?= deploy/charts/mariadb-operator
CLUSTER ?= mdb

##@ Cluster

KIND_CONFIG ?= hack/config/kind.yaml
KIND_IMAGE ?= kindest/node:v1.27.3

.PHONY: cluster
cluster: kind ## Create a single node kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --image $(KIND_IMAGE)

.PHONY: cluster-ha
cluster-ha: kind ## Create a HA kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

.PHONY: cluster-delete
cluster-delete: kind ## Delete the kind cluster.
	$(KIND) delete cluster --name $(CLUSTER)

.PHONY: cluster-ctx
cluster-ctx: ## Sets cluster context.
	@kubectl config use-context kind-$(CLUSTER)

.PHONY: cluster-ls
cluster-ps: ## List all cluster Nodes.
	docker ps --filter="name=$(CLUSTER)-*"

.PHONY: cluster-workers
cluster-workers: ## List cluster worker Nodes.
	docker ps --filter="name=$(CLUSTER)-worker-*"

##@ DR

MARIADB_INSTANCE ?= mariadb-galera

stop-mariadb-%: ## Stop mariadb Node
	docker stop $(shell kubectl get pod "$(MARIADB_INSTANCE)-$*" -o jsonpath="{.spec.nodeName}")

start-mariadb-%: ## Stop mariadb Node
	docker start $(shell kubectl get pod "$(MARIADB_INSTANCE)-$*" -o jsonpath="{.spec.nodeName}")

.PHONY: stop-all-mariadb
stop-all-mariadb: ## Stop all mariadb Nodes
	@for ((i=0; i<$(shell kubectl get mariadb "$(MARIADB_INSTANCE)" -o jsonpath='{.spec.replicas}'); i++)); do make -s "stop-mariadb-$$i"; done
	@make -s cluster-workers

.PHONY: start-all-mariadb
start-all-mariadb: ## Stop all mariadb Nodes
	@for ((i=0; i<$(shell kubectl get mariadb "$(MARIADB_INSTANCE)" -o jsonpath='{.spec.replicas}'); i++)); do make -s "start-mariadb-$$i"; done
	@make -s cluster-workers

##@ Controller gen

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: code
code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

##@ Helm

.PHONY: helm-crds 
helm-crds: kustomize ## Generate CRDs for Helm chart.
	$(KUSTOMIZE) build config/crd > deploy/charts/mariadb-operator/crds/crds.yaml

DOCS_IMG ?= jnorwood/helm-docs:v1.11.0
.PHONY: helm-docs
helm-docs: ## Generate Helm chart docs.
	docker run --rm -v $(shell pwd)/$(HELM_DIR):/helm-docs -u $(shell id -u) $(DOCS_IMG)

CT_IMG ?= quay.io/helmpack/chart-testing:v3.5.0 
.PHONY: helm-lint
helm-lint: ## Lint Helm charts.
	docker run --rm --workdir /repo -v $(shell pwd):/repo $(CT_IMG) ct lint --config .github/config/ct.yml 

.PHONY: helm
helm: helm-crds helm-docs ## Generate manifests for Helm chart.

.PHONY: helm-chart-version
helm-chart-version: yq ## Get helm chart version.
	@cat $(HELM_DIR)/Chart.yaml | $(YQ) e ".version"

##@ Manifests

MANIFESTS_CRDS_DIR ?= deploy/crds
.PHONY: manifests-crds
manifests-crds: manifests kustomize ## Generate manifests CRDs.
	mkdir -p $(MANIFESTS_CRDS_DIR)
	$(KUSTOMIZE) build config/crd > $(MANIFESTS_CRDS_DIR)/crds.yaml

MANIFESTS_DIR ?= deploy/manifests

MANIFESTS_BUNDLE_VALUES ?= deploy/manifests/helm-values.yaml 
.PHONY: manifests-bundle-helm
manifests-bundle-helm: manifests manifests-crds ## Generate manifests bundle from helm chart.
	mkdir -p $(MANIFESTS_DIR)
	cat $(MANIFESTS_CRDS_DIR)/crds.yaml > $(MANIFESTS_DIR)/manifests.yaml
	helm template -n default mariadb-operator $(HELM_DIR) -f $(MANIFESTS_BUNDLE_VALUES) >> $(MANIFESTS_DIR)/manifests.yaml

MANIFESTS_BUNDLE_MIN_VALUES ?= deploy/manifests/helm-values.min.yaml 
.PHONY: manifests-bundle-helm-min
manifests-bundle-helm-min: manifests manifests-crds ## Generate minimal manifests bundle.
	mkdir -p $(MANIFESTS_DIR)
	cat $(MANIFESTS_CRDS_DIR)/crds.yaml > $(MANIFESTS_DIR)/manifests.min.yaml
	helm template -n default mariadb-operator $(HELM_DIR) -f $(MANIFESTS_BUNDLE_MIN_VALUES) >> $(MANIFESTS_DIR)/manifests.min.yaml

.PHONY: manifests-bundle
manifests-bundle: manifests-crds manifests-bundle-helm manifests-bundle-helm-min ## Generate manifests.

##@ Generate

.PHONY: generate
generate: manifests code helm manifests-bundle ## Generate manifests, code, helm chart and manifests bundle.

.PHONY: gen
gen: generate ## Generate alias.

##@ Dependencies

PROMETHEUS_VERSION ?= kube-prometheus-stack-33.2.0
.PHONY: install-prometheus-crds
install-prometheus-crds: cluster-ctx  ## Install Prometheus CRDs.
	kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/$(PROMETHEUS_VERSION)/charts/kube-prometheus-stack/crds/crd-servicemonitors.yaml

.PHONY: install-prometheus
install-prometheus: cluster-ctx ## Install kube-prometheus-stack helm chart.
	@./hack/install_prometheus.sh

CERT_MANAGER_VERSION ?= "v1.9.1"
.PHONY: install-cert-manager
install-cert-manager: cluster-ctx ## Install cert-manager helm chart.
	@./hack/install_cert_manager.sh

METALLB_VERSION ?= "0.13.9"
.PHONY: install-metallb
install-metallb: cluster-ctx ## Install metallb helm chart.
	@./hack/install_metallb.sh

##@ Install

.PHONY: install-crds
install-crds: cluster-ctx manifests kustomize ## Install CRDs.
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true --force-conflicts -f -

.PHONY: uninstall-crds
uninstall-crds: cluster-ctx manifests kustomize ## Uninstall CRDs.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: install
install: cluster-ctx install-crds install-prometheus-crds install-samples serviceaccount cert ## Install CRDs and dependencies for local development.

.PHONY: install-samples
install-samples: cluster-ctx  ## Install sample configuration.
	kubectl apply -f examples/manifests/config

.PHONY: serviceaccount
serviceaccount: cluster-ctx  ## Create long-lived ServiceAccount token for development.
	@./hack/create_serviceaccount.sh

##@ Examples

GITHUB_USER := mariadb-operator
GITHUB_REPOSITORY := mariadb-operator
GITHUB_BRANCH ?= main

.PHONY: example-flux
example-flux: flux ## Install flux example.
	flux bootstrap github \
		--owner=$(GITHUB_USER) \
		--repository=$(GITHUB_REPOSITORY)\
		--branch=$(GITHUB_BRANCH) \
		--path=./examples/flux/clusters/production \
		--personal