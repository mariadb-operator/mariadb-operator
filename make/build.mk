##@ Build

IMAGE_TAG_BASE ?= ghcr.io/mariadb-operator/mariadb-operator
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)

.PHONY: build
build: ## Build binary.
	go build -o bin/mariadb-operator cmd/controller/*.go

.PHONY: docker-build
docker-build: ## Build docker image.
	docker build -t $(IMG) .  

.PHONY: docker-push
docker-push: ## Push docker image.
	docker push $(IMG)

.PHONY: docker-load
docker-load: ## Load docker image in KIND.
	$(KIND) load docker-image --name $(CLUSTER) $(IMG)