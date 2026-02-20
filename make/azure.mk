##@ Azure

AZURE_STORAGE_ACCOUNT_NAME ?= devstoreaccount1
AZURE_STORAGE_ACCOUNT_KEY ?= Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==
AZURE_SERVICE_URL ?= https://172.18.0.60:10000/devstoreaccount1

.PHONY: azurite-seed-containers
azurite-seed-containers: host-azurite ## Seeds development containers in azurite
	AZURE_STORAGE_ACCOUNT_NAME=$(AZURE_STORAGE_ACCOUNT_NAME) AZURE_STORAGE_ACCOUNT_KEY=$(AZURE_STORAGE_ACCOUNT_KEY) AZURE_SERVICE_URL=$(AZURE_SERVICE_URL) $(GO) run ./hack/azurite/main.go

AZURITE_DB ?= $(KUBECTL) exec deployment/azurite -- cat /data/__azurite_db_blob__.json 

.PHONY: azurite-debug
azurite-debug: kubectl jq ## Show azurite internal state.
	@$(AZURITE_DB) | $(JQ)

.PHONY: azurite-blobs
azurite-blobs: kubectl jq ## Lists azurite blobs.
	@$(AZURITE_DB) | $(JQ) -r '.collections[] | select(.name == "$$BLOBS_COLLECTION$$") | .data[] | "\(.name) @ \(.containerName)"'

.PHONY: azurite-containers
azurite-containers: kubectl jq ## Lists azurite containers.
	@$(AZURITE_DB) | $(JQ) -r '.collections[] | select(.name == "$$CONTAINERS_COLLECTION$$") | .data[] | "\(.name) @ \(.accountName)"'

.PHONY: install-azurite
install-azurite: kubectl ## Sets up Azurite for local development
	$(KUBECTL) apply -k ./hack/manifests/azurite/

	@if ! $(KUBECTL) get secret azurite-certs >/dev/null 2>&1; then \
			echo "Certificates not found. Generating..."; \
			openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout key.pem -out cert.pem -config ./hack/manifests/azurite/tls.config; \
			$(KUBECTL) create secret generic azurite-certs --from-file=cert.pem --from-file=key.pem; \
			rm -rf cert.pem key.pem; \
	else \
			echo "Secret 'azurite-certs' already exists. Skipping generation."; \
	fi
	$(KUBECTL) wait deployment.apps/azurite --for condition=Available --timeout 2m
	$(MAKE) azurite-seed-containers

.PHONY: uninstall-azurite
uninstall-azurite:  ## Removes azurite
	$(KUBECTL) delete -k ./hack/manifests/azurite/

