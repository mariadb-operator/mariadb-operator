HELM_DIR ?= deploy/charts/mariadb-operator

##@ Cluster

CLUSTER ?= mdb
KIND_IMAGE ?= kindest/node:v1.26.0
.PHONY: cluster
cluster: kind ## Create the kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --image $(KIND_IMAGE)

.PHONY: cluster-delete
cluster-delete: kind ## Delete the kind cluster.
	$(KIND) delete cluster --name $(CLUSTER)

.PHONY: cluster-ctx
cluster-ctx: ## Sets cluster context.
	@kubectl config use-context kind-$(CLUSTER)

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

.PHONY: helm-rbac
helm-rbac: kustomize ## Generate RBAC for Helm chart.
	$(KUSTOMIZE) build config/rbac | sed 's/namespace: mariadb-system/namespace: {{ .Release.Namespace }}/g' > deploy/charts/mariadb-operator/templates/rbac.yaml

DOCS_IMG ?= jnorwood/helm-docs:v1.11.0
.PHONY: helm-docs
helm-docs: ## Generate Helm chart docs.
	docker run --rm -v $(shell pwd)/$(HELM_DIR):/helm-docs -u $(shell id -u) $(DOCS_IMG)

CT_IMG ?= quay.io/helmpack/chart-testing:v3.5.0 
.PHONY: helm-lint
helm-lint: ## Lint Helm charts.
	docker run --rm --workdir /repo -v $(shell pwd):/repo $(CT_IMG) ct lint --config .github/config/ct.yml 

.PHONY: helm
helm: helm-crds helm-rbac helm-docs ## Generate manifests for Helm chart.

##@ Bundle

BUNDLE_CRDS_DIR ?= deploy/crds
.PHONY: bundle-crds
bundle-crds: manifests kustomize ## Generate CRDs bundle.
	mkdir -p $(BUNDLE_CRDS_DIR)
	$(KUSTOMIZE) build config/crd > $(BUNDLE_CRDS_DIR)/crds.yaml

BUNDLE_VALUES ?= deploy/manifests/helm-values.yaml 
BUNDLE_MANIFESTS_DIR ?= deploy/manifests
.PHONY: bundle-manifests
bundle-manifests: manifests ## Generate manifests bundle.
	mkdir -p $(BUNDLE_MANIFESTS_DIR)
	helm template -n default mariadb-operator $(HELM_DIR) -f $(BUNDLE_VALUES) > $(BUNDLE_MANIFESTS_DIR)/manifests.yaml

.PHONY: bundle
bundle: bundle-crds bundle-manifests ## Generate bundles.

##@ Generate

.PHONY: generate
generate: manifests code helm bundle ## Generate manifests, code, helm chart and manifests bundle.

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

##@ Deploy

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: cluster-ctx manifests kustomize install-prometheus-crds install-samples certs ## Install dependencies to run locally.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: install-samples
install-samples: cluster-ctx  ## Install sample configuration.
	kubectl apply -f config/samples/config

.PHONY: uninstall
uninstall: cluster-ctx manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: cluster-ctx manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: cluster-ctx ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

