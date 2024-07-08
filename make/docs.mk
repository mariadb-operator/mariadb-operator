##@ Documentation

DOCS_HELM_IMG ?= jnorwood/helm-docs:v1.11.0
.PHONY: docs-helm
docs-helm: ## Generate Helm chart docs.
	docker run --rm -v $(shell pwd)/$(HELM_DIR):/helm-docs -u $(shell id -u) $(DOCS_HELM_IMG)

.PHONY: docs-api
docs-api: crd-ref-docs ## Generate API reference docs.
	$(CRD_REF_DOCS) \
		--source-path=./api/v1alpha1 \
		--config=./hack/config/crd-ref-docs.yaml \
		--renderer=markdown \
		--output-path=./docs/API_REFERENCE.md

.PHONY: docs-toc
docs-toc: mdtoc ## Generate table of contents in docs.
	@for f in $$(ls docs/*.md | grep -v 'API_REFERENCE' | grep -v 'UPGRADE'); do \
		$(MDTOC) --inplace $$f; \
	done

MKDOCS_IMG ?= squidfunk/mkdocs-material:9.5.27
MKDOCS ?= docker run --rm -it \
	-u $(shell id -u):$(shell id -g) \
	-v $(shell pwd):/docs \
	-p 8000:8000 \
	$(MKDOCS_IMG)

.PHONY: docs-new
docs-new: ## Create new documentation site.
	$(MKDOCS) new .

.PHONY: docs-serve
docs-serve: ## Serve documentation site locally for development.
	$(MKDOCS)

.PHONY: docs-build
docs-build: ## Build documentation site.
	$(MKDOCS) build

.PHONY: docs
docs: docs-helm docs-api docs-toc ## Generate docs.