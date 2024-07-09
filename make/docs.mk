##@ Documentation

HELM_DOCS_IMG ?= jnorwood/helm-docs:v1.11.0
.PHONY: docs-helm
docs-helm: ## Generate Helm chart docs.
	docker run --rm \
		-u $(shell id -u) \
		-v $(shell pwd)/$(HELM_DIR):/helm-docs \
		$(HELM_DOCS_IMG)

.PHONY: docs-api
docs-api: crd-ref-docs ## Generate API reference docs.
	$(CRD_REF_DOCS) \
		--source-path=./api/v1alpha1 \
		--config=./hack/config/crd-ref-docs.yaml \
		--renderer=markdown \
		--output-path=./docs/API_REFERENCE.md

.PHONY: docs-gen
docs-gen: docs-helm docs-api ## Generate docs.

DOCS_IMG ?= mariadb-operator/docs:0.0.1
DOCS_RUN ?= docker run --rm \
	-u $(shell id -u):$(shell id -g) \
	-v $(shell pwd):/docs \
	-p 8000:8000 \
	-e "GIT_COMMITTER_NAME=$(shell git config user.name)" \
	-e "GIT_COMMITTER_EMAIL=$(shell git config user.email)" \
	$(DOCS_IMG)
MKDOCS ?= $(DOCS_RUN) mkdocs
MIKE ?= $(DOCS_RUN) mike

.PHONY: docs-image
docs-image: ## Build a new docs image
	docker build -t $(DOCS_IMG) -f docs/Dockerfile docs

.PHONY: docs-new
docs-new: docs-image ## Create new documentation site.
	$(MKDOCS) new .

.PHONY: docs-serve
docs-serve: docs-image ## Serve documentation site locally for development.
	$(MKDOCS) serve --dev-addr=0.0.0.0:8000

.PHONY: docs-build
docs-build: docs-image ## Build documentation site.
	$(MKDOCS) build

DOCS_VERSION ?= main
DOCS_ALIAS ?= unstable
.PHONY: docs-publish
docs-publish: docs-image ## Publish documentation site.
	$(MIKE) deploy --push --update-aliases $(DOCS_VERSION) $(DOCS_ALIAS)

DOCS_DEFAULT ?= latest
.PHONY: docs-set-default
docs-set-default: docs-image ## Set documentation default version.
	$(MIKE) set-default --push $(DOCS_DEFAULT)