##@ Build

DOCKER_ARGS ?= --load

.PHONY: build
build: ## Build binary.
	$(GO) build -o bin/mariadb-operator cmd/controller/*.go

.PHONY: docker-build
docker-build: ## Build docker image.
	$(DOCKER) buildx build -t $(IMG) . $(DOCKER_ARGS)

.PHONY: docker-load
docker-load: kind ## Load docker image in KIND.
	$(KIND) load docker-image --name $(CLUSTER) $(IMG)

.PHONY: docker-dev
docker-dev: docker-build docker-load ## Build and load docker image for local development.