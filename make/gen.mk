##@ Generate - Controller gen

KUBE_API_VERSION ?= v1.33
.PHONY: kube-api-version
kube-api-version: ## Update Kubernetes version in links available in API docs.
	@KUBE_API_VERSION=$(KUBE_API_VERSION) ./hack/kube_api_version.sh

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
	@./hack/bump_version_examples.sh examples/manifests $(IMG_NAME) $(VERSION)

.PHONY: examples-mariadb
examples-mariadb: ## Update mariadb version in examples
	@./hack/bump_version_examples.sh examples/manifests $(RELATED_IMAGE_MARIADB_NAME) $(RELATED_IMAGE_MARIADB_VERSION)

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

##@ Generate - CRD size

CRD_FILE ?= deploy/charts/mariadb-operator-crds/templates/crds.yaml
CRD_MAX_SIZE ?= 900 # in KB
.PHONY: crd-size
crd-size: ## Check CRD size and fail if it exceeds 900KB (hard limit 1MB).
	@max_size=$$((${CRD_MAX_SIZE} * 1024)); \
	crd_size=$$(stat -c%s "${CRD_FILE}"); \
	echo "Current CRD size: $$((crd_size / 1024)) KB"; \
	if [ "$$crd_size" -ge "$$max_size" ]; then \
		echo "Error: CRDs exceed 900KB (current size: $$((crd_size / 1024)) KB)"; \
		exit 1; \
	fi

##@ Generate

.PHONY: gen
ifneq ($(findstring -dev,$(VERSION)),)
gen: kube-api-version manifests code embed-entrypoint helm-crds 
else
gen: kube-api-version manifests code embed-entrypoint helm-gen manifests-bundle docs examples
endif