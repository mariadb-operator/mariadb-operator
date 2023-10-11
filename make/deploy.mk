HELM_DIR ?= deploy/charts/mariadb-operator
CLUSTER ?= mdb

##@ Docker

PLATFORM ?= linux/amd64,linux/arm64
IMG ?= ghcr.io/mariadb-operator/mariadb-operator:v0.0.21
BUILDX ?= docker buildx build --platform $(PLATFORM) -t $(IMG) 
BUILDER ?= mariadb-operator

.PHONY: docker-builder
docker-builder: ## Configure docker builder.
	docker buildx create --name $(BUILDER) --use --platform $(PLATFORM)

.PHONY: docker-build
docker-build: ## Build docker image.
	docker build -t $(IMG) .  

.PHONY: docker-buildx
docker-buildx: ## Build multi-arch docker image.
	$(BUILDX) .

.PHONY: docker-push
docker-push: ## Build multi-arch docker image and push it to the registry.
	$(BUILDX) --push .

.PHONY: docker-inspect
docker-inspect: ## Inspect docker image.
	docker buildx imagetools inspect $(IMG)

.PHONY: docker-load
docker-load: ## Load docker image in KIND.
	$(KIND) load docker-image --name ${CLUSTER} ${IMG}

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
	$(CONTROLLER_GEN) rbac:roleName=mariadb-manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

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

##@ Bundle

BUNDLE_CRDS_DIR ?= deploy/crds
.PHONY: bundle-crds
bundle-crds: manifests kustomize ## Generate CRDs bundle.
	mkdir -p $(BUNDLE_CRDS_DIR)
	$(KUSTOMIZE) build config/crd > $(BUNDLE_CRDS_DIR)/crds.yaml

BUNDLE_MANIFESTS_DIR ?= deploy/manifests

BUNDLE_VALUES ?= deploy/manifests/helm-values.yaml 
.PHONY: bundle-manifests
bundle-manifests: manifests bundle-crds ## Generate manifests bundle.
	mkdir -p $(BUNDLE_MANIFESTS_DIR)
	cat $(BUNDLE_CRDS_DIR)/crds.yaml > $(BUNDLE_MANIFESTS_DIR)/manifests.yaml
	helm template -n default mariadb-operator $(HELM_DIR) -f $(BUNDLE_VALUES) >> $(BUNDLE_MANIFESTS_DIR)/manifests.yaml

BUNDLE_MIN_VALUES ?= deploy/manifests/helm-values.min.yaml 
.PHONY: bundle-min-manifests
bundle-min-manifests: manifests bundle-crds ## Generate minimal manifests bundle.
	mkdir -p $(BUNDLE_MANIFESTS_DIR)
	cat $(BUNDLE_CRDS_DIR)/crds.yaml > $(BUNDLE_MANIFESTS_DIR)/manifests.min.yaml
	helm template -n default mariadb-operator $(HELM_DIR) -f $(BUNDLE_MIN_VALUES) >> $(BUNDLE_MANIFESTS_DIR)/manifests.min.yaml

.PHONY: bundle
bundle: bundle-crds bundle-manifests bundle-min-manifests ## Generate bundles.

##@ Generate

.PHONY: generate
generate: manifests code helm bundle ## Generate manifests, code, helm chart and manifests bundle.

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

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install-crds
install-crds: cluster-ctx manifests kustomize ## Install CRDs.
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true --force-conflicts -f -

.PHONY: uninstall-crds
uninstall-crds: cluster-ctx manifests kustomize ## Uninstall CRDs.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: install
install: cluster-ctx install-crds install-prometheus-crds install-samples serviceaccount certs ## Install CRDs and dependencies for local development.

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