##@ Build

DOCKER_PLATFORMS ?= linux/amd64
DOCKER_ARGS ?=

.PHONY: build
build: ## Build binary.
	go build -o bin/mariadb-operator cmd/controller/*.go

.PHONY: docker-build
docker-build: ## Build docker image.
	docker buildx build --platform $(DOCKER_PLATFORMS) -t $(IMG) . $(DOCKER_ARGS)

.PHONY: docker-load
docker-load: kind ## Load docker image in KIND.
	$(KIND) load docker-image --name $(CLUSTER) $(IMG)

.PHONY: docker-dev
docker-dev: docker-build docker-load ## Build and load docker image for local development.

##@ Build Enterprise

.PHONY: build-ent
build-ent: ## Build the enterprise binary.
	go build -o bin/mariadb-operator-enterprise cmd/enterprise/*.go

.PHONY: docker-build-ent
docker-build-ent: ## Build the enterprise image.
	docker buildx build --platform $(DOCKER_PLATFORMS) -f Dockerfile.ent -t $(IMG_ENT) . $(DOCKER_ARGS)

.PHONY: docker-load-ent
docker-load-ent: ## Load the enterprise image in KIND.
	$(MAKE) docker-load IMG=$(IMG_ENT)

.PHONY: docker-dev-ent
docker-dev-ent: docker-build-ent docker-load-ent ## Build and load enterprise image for local development.