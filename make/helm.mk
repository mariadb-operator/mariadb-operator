##@ Helm

HELM_DIR ?= deploy/charts/mariadb-operator
HELM_CHART_FILE ?= $(HELM_DIR)/Chart.yaml
HELM_VALUES_FILE ?= $(HELM_DIR)/values.yaml

HELM_CRDS_DIR ?= deploy/charts/mariadb-operator-crds
HELM_CRDS_CHART_FILE ?= $(HELM_CRDS_DIR)/Chart.yaml

HELM_CT_IMG ?= quay.io/helmpack/chart-testing:v3.5.0 
HELM_CT_CONFIG ?= hack/config/chart-testing/mariadb-operator.yaml
HELM_CRDS_CT_CONFIG ?= hack/config/chart-testing/mariadb-operator-crds.yaml
.PHONY: helm-lint
helm-lint: ## Lint mariadb-operator helm chart.
	$(DOCKER) run --rm --workdir /repo -v $(shell pwd):/repo $(HELM_CT_IMG) ct lint --config $(HELM_CT_CONFIG) 

.PHONY: helm-crds-lint
helm-crds-lint: ## Lint mariadb-operator-crds helm chart.
	$(DOCKER) run --rm --workdir /repo -v $(shell pwd):/repo $(HELM_CT_IMG) ct lint --config $(HELM_CRDS_CT_CONFIG) 

.PHONY: helm-crds 
helm-crds: kustomize ## Generate CRDs for the Helm chart.
	$(KUSTOMIZE) build config/crd > $(HELM_CRDS_DIR)/templates/crds.yaml
	helm dependency update deploy/charts/mariadb-operator

.PHONY: helm-config
helm-config: yq ## Update operator config in the Helm chart.
	$(YQ) e -i '.config.galeraLibPath = "$(MARIADB_GALERA_LIB_PATH)"' $(HELM_VALUES_FILE)
	$(YQ) e -i '.config.mariadbDefaultVersion = "$(MARIADB_DEFAULT_VERSION)"' $(HELM_VALUES_FILE)
	$(YQ) e -i '.config.mariadbImage = "$(RELATED_IMAGE_MARIADB)"' $(HELM_VALUES_FILE)
	$(YQ) e -i '.config.maxscaleImage = "$(RELATED_IMAGE_MAXSCALE)"' $(HELM_VALUES_FILE)
	$(YQ) e -i '.config.exporterImage = "$(RELATED_IMAGE_EXPORTER)"' $(HELM_VALUES_FILE)
	$(YQ) e -i '.config.exporterMaxscaleImage = "$(RELATED_IMAGE_EXPORTER_MAXSCALE)"' $(HELM_VALUES_FILE)

HELM_DOCS_IMG ?= jnorwood/helm-docs:v1.14.2
.PHONY: helm-docs
helm-docs: ## Generate Helm chart docs.
	$(DOCKER) run --rm \
		-u $(shell id -u) \
		-v $(shell pwd)/$(HELM_DIR):/helm-docs \
		$(HELM_DOCS_IMG)

.PHONY: helm-gen
helm-gen: helm-crds helm-config helm-docs ## Generate manifests and documentation for the Helm chart.

.PHONY: helm-version
helm-version: yq ## Get mariadb-operator chart version.
ifndef HELM_CHART_FILE
	$(error HELM_CHART_FILE is not set. Please set it before running this target)
endif
	@cat $(HELM_CHART_FILE) | $(YQ) e ".version"

.PHONY: helm-crds-version
helm-crds-version: yq ## Get mariadb-operator-crds chart version.
ifndef HELM_CRDS_CHART_FILE
	$(error HELM_CRDS_CHART_FILE is not set. Please set it before running this target)
endif
	@cat $(HELM_CRDS_CHART_FILE) | $(YQ) e ".version"

HELM_VERSION ?=
.PHONY: helm-version-bump
helm-version-bump: yq ## Bump helm charts version.
ifndef HELM_CHART_FILE
	$(error HELM_CHART_FILE is not set. Please set it before running this target)
endif
ifndef HELM_CRDS_CHART_FILE
	$(error HELM_CRDS_CHART_FILE is not set. Please set it before running this target)
endif
ifndef HELM_VERSION
	$(error HELM_VERSION is not set. Please set it before running this target)
endif
	@$(YQ) e -i ".version = \"$(HELM_VERSION)\"" $(HELM_CHART_FILE); \
	$(YQ) e -i ".appVersion = \"$(HELM_VERSION)\"" $(HELM_CHART_FILE); \
	$(YQ) e -i ".dependencies |= map(select(.name == \"mariadb-operator-crds\").version = \"$(HELM_VERSION)\" // .)" $(HELM_CHART_FILE); \
	$(YQ) e -i ".version = \"$(HELM_VERSION)\"" $(HELM_CRDS_CHART_FILE)
