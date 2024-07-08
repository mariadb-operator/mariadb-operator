##@ Documentation

DOCS_IMG ?= jnorwood/helm-docs:v1.11.0
.PHONY: docs-helm
docs-helm: ## Generate Helm chart docs.
	docker run --rm -v $(shell pwd)/$(HELM_DIR):/helm-docs -u $(shell id -u) $(DOCS_IMG)

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

.PHONY: docs
docs: docs-helm docs-api docs-toc ## Generate docs.