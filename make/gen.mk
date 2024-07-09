##@ Generate - Controller gen

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: code
code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

##@ Generate - Embed

.PHONY: embed-entrypoint
embed-entrypoint: ## Get entrypoint from mariadb-docker to be embeded in operator binary. See: https://github.com/MariaDB/mariadb-docker/blob/master/docker-entrypoint.sh.
	@MARIADB_DOCKER_REPO=$(MARIADB_DOCKER_REPO) \
	MARIADB_DOCKER_COMMIT_HASH=$(MARIADB_DOCKER_COMMIT_HASH) \
	MARIADB_ENTRYPOINT_PATH=$(MARIADB_ENTRYPOINT_PATH) \
	./hack/get_entrypoint.sh

##@ Generate - Helm

.PHONY: helm-crds 
helm-crds: kustomize ## Generate CRDs for Helm chart.
	$(KUSTOMIZE) build config/crd > deploy/charts/mariadb-operator/crds/crds.yaml

.PHONY: helm-env
helm-env: ## Update operator env in the Helm chart.
	$(KUBECTL) create configmap mariadb-operator-env \
		--from-literal=RELATED_IMAGE_MARIADB=$(RELATED_IMAGE_MARIADB) \
		--from-literal=RELATED_IMAGE_MAXSCALE=$(RELATED_IMAGE_MAXSCALE) \
		--from-literal=RELATED_IMAGE_EXPORTER=$(RELATED_IMAGE_EXPORTER) \
		--from-literal=RELATED_IMAGE_EXPORTER_MAXSCALE=$(RELATED_IMAGE_EXPORTER_MAXSCALE) \
		--from-literal=MARIADB_OPERATOR_IMAGE=$(IMG) \
		--from-literal=MARIADB_GALERA_INIT_IMAGE=$(MARIADB_GALERA_INIT_IMAGE) \
		--from-literal=MARIADB_GALERA_AGENT_IMAGE=$(MARIADB_GALERA_AGENT_IMAGE) \
		--from-literal=MARIADB_GALERA_LIB_PATH=$(MARIADB_GALERA_LIB_PATH) \
		--from-literal=MARIADB_ENTRYPOINT_VERSION=$(MARIADB_ENTRYPOINT_VERSION) \
		--dry-run=client -o yaml \
		> deploy/charts/mariadb-operator/templates/configmap.yaml

.PHONY: helm
helm: helm-crds helm-env ## Generate manifests for Helm chart.

##@ Generate - Manifests

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

##@ Generate - Examples

.PHONY: examples-operator
examples-operator: ## Update mariadb-operator version in examples
	@./hack/bump_version_examples.sh examples/manifests $(IMG_NAME) $(IMG_VERSION)
	@./hack/bump_version_examples.sh config/samples $(IMG_ENT_NAME) $(IMG_ENT_VERSION)

.PHONY: examples-mariadb
examples-mariadb: ## Update mariadb version in examples
	@./hack/bump_version_examples.sh examples/manifests $(RELATED_IMAGE_MARIADB_NAME) $(RELATED_IMAGE_MARIADB_VERSION)
	@./hack/bump_version_examples.sh config/samples $(RELATED_IMAGE_MARIADB_ENT_NAME) $(RELATED_IMAGE_MARIADB_ENT_VERSION)

.PHONY: examples-maxscale
examples-maxscale: ## Update maxscale version in examples
	@./hack/bump_version_examples.sh examples/manifests $(RELATED_IMAGE_MAXSCALE_NAME) $(RELATED_IMAGE_MAXSCALE_VERSION)
	@./hack/bump_version_examples.sh config/samples $(RELATED_IMAGE_MAXSCALE_NAME) $(RELATED_IMAGE_MAXSCALE_VERSION)

.PHONY: examples-exporter
examples-exporter: ## Update exporter version in examples
	@./hack/bump_version_examples.sh examples/manifests $(RELATED_IMAGE_EXPORTER_NAME) $(RELATED_IMAGE_EXPORTER_VERSION)
	@./hack/bump_version_examples.sh config/samples $(RELATED_IMAGE_EXPORTER_NAME) $(RELATED_IMAGE_EXPORTER_VERSION)

.PHONY: examples
examples: examples-operator examples-mariadb examples-maxscale examples-exporter ## Update versions in examples

##@ Generate

.PHONY: generate
ifneq ($(findstring -dev,$(VERSION)),)
generate: manifests code embed-entrypoint
else
generate: manifests code embed-entrypoint helm manifests-bundle docs-gen examples
endif

.PHONY: gen
gen: generate ## Generate alias.