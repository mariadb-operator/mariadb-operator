##@ OLM

ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

BUNDLE_GEN_FLAGS ?= -q --overwrite=false --version $(VERSION) $(BUNDLE_METADATA_OPTS)
USE_IMAGE_DIGESTS ?= true
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

BUNDLE_IMG ?= mariadb/mariadb-operator-enterprise-bundle:v$(VERSION)
BUNDLE_IMGS ?= $(BUNDLE_IMG)

CATALOG_IMG ?= mariadb/mariadb-operator-enterprise-catalog:v$(VERSION)
CATALOG_REGISTRY_CONF ?= $(HOME)/.docker/config.json
CATALOG_REGISTRY_URL ?= https://index.docker.io/v1/
CATALOG_REGISTRY_AUTH = $(shell cat $(CATALOG_REGISTRY_CONF) | $(JQ) '.auths["$(CATALOG_REGISTRY_URL)"]')

.PHONY: scorecard-sa
scorecard-sa: ## Create scorecard ServiceAccount.
	$(KUBECTL) apply -f ./hack/manifests/scorecard-serviceaccount.yaml

BUNDLE_VALIDATE_FLAGS ?= --select-optional suite=operatorframework
.PHONY: bundle-validate
bundle-validate: operator-sdk ## Validate content and format of the operator bundle.
	$(OPERATOR_SDK) bundle validate ./bundle $(BUNDLE_VALIDATE_FLAGS)

BUNBLDE_SCORECARD_FLAGS ?= --service-account=scorecard --namespace=default --wait-time=3m
.PHONY: bundle-scorecard
bundle-scorecard: operator-sdk cluster-ctx scorecard-sa ## Statically validate your operator bundle using Scorecard.
	$(OPERATOR_SDK) scorecard ./bundle $(BUNBLDE_SCORECARD_FLAGS)

.PHONY: bundle
bundle: operator-sdk yq kustomize manifests ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(ENT_IMG)
	$(YQ) e -i '.spec.template.spec.containers[0].env[0].value = "$(RELATED_IMAGE_MARIADB)"' config/manager/manager.yaml
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(YQ) e -i '.metadata.annotations.containerImage = (.spec.relatedImages[] | select(.name == "mariadb-operator-enterprise").image)' bundle/manifests/mariadb-operator-enterprise.clusterserviceversion.yaml
	$(MAKE) bundle-validate

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

.PHONY: catalog-registry-login
catalog-registry-login: ## Login in the catalog registry.
	docker login $(CATALOG_REGISTRY_URL)

.PHONY: catalog-config
catalog-config: oc jq catalog-registry-login ## Setup catalog registry credentials in OpenShift global config.
	$(OC) extract secret/pull-secret -n openshift-config --confirm
	@cat .dockerconfigjson | $(JQ) -c --argjson registryauth '$(CATALOG_REGISTRY_AUTH)' '.auths["$(CATALOG_REGISTRY_URL)"] |= . + $$registryauth' > .new_dockerconfigjson 
	$(OC) set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=.new_dockerconfigjson 
	@rm .dockerconfigjson .new_dockerconfigjson 

.PHONY: catalog-deploy
catalog-deploy: catalog-config ## Deploy catalog to a OpenShift cluster.
	cd hack/manifests/catalog && $(KUSTOMIZE) edit set image catalog=$(CATALOG_IMG)
	$(KUSTOMIZE) build hack/manifests/catalog	| $(KUBECTL) apply -f -

.PHONY: catalog-undeploy
catalog-undeploy: ## Undeploy catalog from a OpenShift cluster.
	$(KUSTOMIZE) build hack/manifests/catalog	| $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -