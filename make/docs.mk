##@ Documentation

.PHONY: docs-api
docs-api: crd-ref-docs ## Generate API reference docs.
	$(CRD_REF_DOCS) \
		--source-path=./api/v1alpha1 \
		--config=./hack/config/crd-ref-docs.yaml \
		--renderer=markdown \
		--output-path=./docs/API_REFERENCE.md

.PHONY: docs-docker
docs-docker: ## Generate Docker docs.
	VERSION=$(VERSION) $(GO) run ./hack/render_docker_docs/main.go

.PHONY: docs
docs: docs-api docs-docker ## Generate documentation.