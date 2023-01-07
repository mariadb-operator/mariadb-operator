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

.PHONY: codegen
codegen: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

##@ Helm

.PHONY: helmcrds 
helm-crds: kustomize ## Generate CRDs for Helm chart.
	$(KUSTOMIZE) build config/crd > deploy/mariadb-operator/crds/crds.yaml

.PHONY: helm-rbac
helm-rbac: kustomize ## Generate RBAC for Helm chart.
	$(KUSTOMIZE) build config/rbac | sed 's/namespace: mariadb-system/namespace: {{ .Release.Namespace }}/g' > deploy/mariadb-operator/templates/rbac.yaml

.PHONY: helm
helm: helm-crds helm-rbac ## Generate manifests for Helm chart.

##@ Generate

.PHONY: generate
generate: manifests codegen helm ## Generate manifests, code and helm chart.

##@ Deploy

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: cluster-ctx manifests kustomize install-prometheus-crds install-samples certs ## Install dependencies to run locally.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

PROMETHEUS_VERSION ?= kube-prometheus-stack-33.2.0
.PHONY: install-prometheus-crds
install-prometheus-crds: cluster-ctx  ## Install Prometheus CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/$(PROMETHEUS_VERSION)/charts/kube-prometheus-stack/crds/crd-servicemonitors.yaml

.PHONY: install-prometheus
install-prometheus: cluster-ctx ## Install kube-prometheus-stack helm chart.
	@./hack/install_prometheus.sh

.PHONY: install-cert-manager
install-cert-manager: cluster-ctx ## Install cert-manager helm chart.
	@./hack/install_cert_manager.sh

.PHONY: install-samples
install-samples: cluster-ctx  ## Install sample configuration.
	kubectl apply -f config/samples/config

.PHONY: uninstall
uninstall: cluster-ctx manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: cluster-ctx manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: cluster-ctx ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

