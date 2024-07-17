##@ Helm

HELM_DIR ?= deploy/charts/mariadb-operator
HELM_CHART_FILE ?= $(HELM_DIR)/Chart.yaml

.PHONY: helm-crds 
helm-crds: kustomize ## Generate CRDs for the Helm chart.
	$(KUSTOMIZE) build config/crd > $(HELM_DIR)/crds/crds.yaml

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
		> $(HELM_DIR)/templates/configmap.yaml

HELM_DOCS_IMG ?= jnorwood/helm-docs:v1.14.2
.PHONY: helm-docs
helm-docs: ## Generate Helm chart docs.
	docker run --rm \
		-u $(shell id -u) \
		-v $(shell pwd)/$(HELM_DIR):/helm-docs \
		$(HELM_DOCS_IMG)

.PHONY: helm-gen
helm-gen: helm-crds helm-env helm-docs ## Generate manifests and documentation for the Helm chart.

.PHONY: helm-version-bump
helm-version-bump: yq ## Bump helm minor version and return it to stdout.
	@if [ ! -f $(HELM_CHART_FILE) ]; then \
		echo "Error: $(HELM_CHART_FILE) not found"; \
		exit 1; \
	fi; \
	VERSION=$$($(YQ) e '.version' $(HELM_CHART_FILE)); \
	MAJOR=$$(echo $$VERSION | cut -d'.' -f1); \
	MINOR=$$(echo $$VERSION | cut -d'.' -f2); \
	NEW_MINOR=$$((MINOR + 1)); \
	NEW_VERSION=$$MAJOR.$$NEW_MINOR.0; \
	$(YQ) e -i ".version = \"$$NEW_VERSION\"" $(HELM_CHART_FILE); \
	echo $$NEW_VERSION