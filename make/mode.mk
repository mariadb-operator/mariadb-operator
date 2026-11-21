##@ Running modes

# In-cluster mode slots get their own ServiceAccount token path, so they never overwrite the one used in host mode.
# They also get their own kubeconfig: the current context of ~/.kube/config is global, so checkouts switching it in parallel
# would run commands against each other's clusters.
ifeq ($(MODE),in-cluster)
MARIADB_OPERATOR_SA_PATH := /tmp/mariadb-operator/$(CLUSTER)/token
KUBECONFIG := /tmp/mariadb-operator/$(CLUSTER)/kubeconfig
export KUBECONFIG
endif

HOST_LEASE = CLUSTER=$(CLUSTER) $(GO) run ./hack/host_lease

.PHONY: cluster-init
cluster-init: ## Bind this checkout to the in-cluster mode slot cluster slot-<SLOT>. Use SLOT=<issue|pr-N>, e.g. SLOT=pr-1234.
ifndef SLOT
	$(error SLOT is required, e.g. 'make cluster-init SLOT=pr-1234')
endif
	@printf 'MODE := in-cluster\nCLUSTER := slot-%s\n' "$$(echo '$(SLOT)' | tr '[:upper:]' '[:lower:]')" > .cluster.mk
	@cat .cluster.mk

.PHONY: cluster-info
cluster-info: ## Print the running mode and cluster of this checkout.
	@echo "MODE=$(MODE) CLUSTER=$(CLUSTER) CONTEXT=kind-$(CLUSTER) KUBECONFIG=$(or $(KUBECONFIG),$(HOME)/.kube/config)"

.PHONY: require-host-mode
require-host-mode:
ifeq ($(MODE),in-cluster)
	$(error GUARD: this target requires host mode, but this checkout is bound to the in-cluster slot '$(CLUSTER)' (.cluster.mk). This is not a bug, do not investigate)
endif
	@$(HOST_LEASE) acquire

.PHONY: require-in-cluster-mode
require-in-cluster-mode:
ifneq ($(MODE),in-cluster)
	$(error GUARD: this target requires in-cluster mode. Run 'make cluster-init' to bind this checkout to a slot cluster. This is not a bug, do not investigate)
endif

.PHONY: host-lease-release
host-lease-release: ## Release the host mode cluster lease held by this checkout.
	@$(HOST_LEASE) release

.PHONY: run-in-cluster
run-in-cluster: require-in-cluster-mode helm cluster-ctx install-crds docker-build docker-load ## Build the operator image, load it into the slot cluster and deploy it with Helm.
	$(HELM) dependency build deploy/charts/mariadb-operator
	$(HELM) upgrade --install mariadb-operator-dev deploy/charts/mariadb-operator --kube-context kind-$(CLUSTER) \
		--set image.repository=$(IMG_NAME) --set image.tag=$(VERSION) --set image.pullPolicy=IfNotPresent
	# The image tag does not change between local rebuilds, so Helm sees no diff: restart to pick up the reloaded image.
	$(KUBECTL) --context kind-$(CLUSTER) rollout restart deployment -l app.kubernetes.io/instance=mariadb-operator-dev
	$(KUBECTL) --context kind-$(CLUSTER) rollout status deployment -l app.kubernetes.io/instance=mariadb-operator-dev --timeout=5m

.PHONY: stop-in-cluster
stop-in-cluster: require-in-cluster-mode helm cluster-ctx ## Uninstall the operator deployed by run-in-cluster.
	$(HELM) uninstall mariadb-operator-dev --kube-context kind-$(CLUSTER) --ignore-not-found

.PHONY: delete-in-cluster
delete-in-cluster: require-in-cluster-mode cluster-delete ## Delete the slot cluster and unbind this checkout from it.
	rm -f .cluster.mk

.PHONY: install-in-cluster
install-in-cluster: require-in-cluster-mode install install-csi-hostpath install-minio install-cert-manager install-azurite ## Install dependencies in the slot cluster.

# Image with the integration tests binary, see Dockerfile.test.
TEST_IMG ?= $(IMG_NAME)-test:$(VERSION)

.PHONY: test-int-in-cluster
test-int-in-cluster: require-in-cluster-mode cluster-ctx envtest kind ## Run integration tests as a Job in the slot cluster. Pass test binary flags with TEST_ARGS, e.g. TEST_ARGS=--ginkgo.focus=Galera.
	$(DOCKER) buildx build -f Dockerfile.test -t $(TEST_IMG) . $(DOCKER_ARGS)
	$(KIND) load docker-image --name $(CLUSTER) $(TEST_IMG)
	CLUSTER=$(CLUSTER) TEST_IMG=$(TEST_IMG) \
		TEST_ENV='$(ENV) AZURE_STORAGE_ACCOUNT_NAME=$(AZURE_STORAGE_ACCOUNT_NAME) AZURE_STORAGE_ACCOUNT_KEY=$(AZURE_STORAGE_ACCOUNT_KEY) AZURE_SERVICE_URL=$(AZURE_SERVICE_URL)' \
		$(GO) run ./hack/test_int_in_cluster --ginkgo.timeout=$(TEST_TIMEOUT) --ginkgo.label-filter='!multi-cluster' $(TEST_ARGS)

.PHONY: test-int-basic-in-cluster
test-int-basic-in-cluster: ## Run integration tests with label 'basic' as a Job in the slot cluster.
	$(MAKE) TEST_ARGS="--ginkgo.label-filter=basic $(TEST_ARGS)" test-int-in-cluster

.PHONY: test-int-multi-cluster-in-cluster
test-int-multi-cluster-in-cluster: ## Run multi-cluster integration tests as a Job in the slot cluster.
	$(MAKE) TEST_ARGS="--ginkgo.label-filter=multi-cluster $(TEST_ARGS)" test-int-in-cluster
