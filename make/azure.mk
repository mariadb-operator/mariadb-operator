##@ Azure

.PHONY: install-azurite
install-azurite: kubectl ## Sets up Azurite for local development
	$(KUBECTL) apply -k ./hack/manifests/azurite/

	@if ! $(KUBECTL) get secret azurite-certs -n azurite >/dev/null 2>&1; then \
			echo "Certificates not found. Generating..."; \
			openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout key.pem -out cert.pem -config ./hack/manifests/azurite/tls.config; \
			$(KUBECTL) create secret generic azurite-certs -n azurite --from-file=cert.pem --from-file=key.pem; \
			rm -rf cert.pem key.pem; \
	else \
			echo "Secret 'azurite-certs' already exists. Skipping generation."; \
	fi

.PHONY: uninstall-azurite
uninstall-azurite:  ## Removes azurite
	$(KUBECTL) delete -k ./hack/manifests/azurite/

