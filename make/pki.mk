##@ PKI

PKI_DIR ?= /tmp/pki

CA_DIR ?= $(PKI_DIR)/ca
CA_CERT ?= $(CA_DIR)/tls.crt
CA_KEY ?= $(CA_DIR)/tls.key
CA_SUBJECT ?= "/CN=mariadb"
.PHONY: ca
ca: ## Generates CA keypair.
	@if [ ! -f "$(CA_CERT)" ] || [ ! -f "$(CA_KEY)" ]; then \
		mkdir -p $(CA_DIR); \
		openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes \
			-out $(CA_CERT) -keyout $(CA_KEY) \
			-subj $(CA_SUBJECT); \
	else \
		echo "CA files already exist, skipping generation."; \
	fi

CA_SECRET_NAME ?= mariadb-ca
CA_SECRET_NAMESPACE ?= default
.PHONY: ca-secret-tls
ca-secret-tls: ## Generates a TLS Secret for a CA.
	$(KUBECTL) create namespace $(CA_SECRET_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret tls $(CA_SECRET_NAME) -n $(CA_SECRET_NAMESPACE) \
		--cert=$(CA_CERT) --key=$(CA_KEY) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -

.PHONY: ca-secret
ca-secret: ## Creates a generic Secret for a CA.
	$(KUBECTL) create namespace $(CA_SECRET_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic $(CA_SECRET_NAME) -n $(CA_SECRET_NAMESPACE) \
		--from-file=$(CA_CERT) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -

CERT ?= $(PKI_DIR)/tls.crt 
KEY ?= $(PKI_DIR)/tls.key 
CERT_SUBJECT ?= "/CN=localhost"
CERT_ALT_NAMES ?= "subjectAltName=DNS:localhost,IP:127.0.0.1"
.PHONY: cert
cert: ## Generates certificate keypair.
	@mkdir -p $(PKI_DIR)
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes \
		-CA $(CA_CERT) -CAkey $(CA_KEY) \
		-out $(CERT) -keyout $(KEY) \
		-subj $(CERT_SUBJECT) -addext $(CERT_ALT_NAMES)

CERT_SECRET_NAME ?= mariadb-tls
CERT_SECRET_NAMESPACE ?= default
.PHONY: cert-secret-tls
cert-secret-tls: kubectl ## Creates a TLS Secret.
	$(KUBECTL) create namespace $(CERT_SECRET_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret tls $(CERT_SECRET_NAME) -n $(CERT_SECRET_NAMESPACE) \
		--cert=$(CERT) --key=$(KEY) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -

WEBHOOK_PKI_DIR ?= /tmp/k8s-webhook-server/serving-certs
.PHONY: cert-webhook
cert-webhook: ca ## Generates webhook private key and certificate for local development.
	PKI_DIR=$(WEBHOOK_PKI_DIR) \
	CA_CERT=$(CA_CERT) \
	CA_KEY=$(CA_KEY) \
	CERT=$(WEBHOOK_PKI_DIR)/tls.crt \
	KEY=$(WEBHOOK_PKI_DIR)/tls.key \
	CERT_SUBJECT="/CN=localhost" \
	CERT_ALT_NAMES="subjectAltName=DNS:localhost,IP:127.0.0.1" \
	$(MAKE) cert

MINIO_PKI_DIR ?= /tmp/pki-minio
MINIO_CERT ?= $(MINIO_PKI_DIR)/tls.crt
MINIO_KEY ?= $(MINIO_PKI_DIR)/tls.key
MINIO_CA_SECRET_NAME ?= minio-ca
MINIO_CA_NAMESPACE ?= default
MINIO_CERT_SECRET_NAME ?= minio-tls
MINIO_CERT_NAMESPACE ?= minio
.PHONY: cert-minio
cert-minio: ca kubectl ## Generates minio private key and certificate for local development.
	PKI_DIR=$(MINIO_PKI_DIR) \
	CA_CERT=$(CA_CERT) \
	CA_KEY=$(CA_KEY) \
	CERT=$(MINIO_CERT) \
	KEY=$(MINIO_KEY) \
	CERT_SUBJECT="/CN=minio.minio.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:minio,DNS:minio.minio,DNS:minio.minio.svc.cluster.local" \
	$(MAKE) cert

	CA_SECRET_NAME=$(MINIO_CA_SECRET_NAME) \
	CA_SECRET_NAMESPACE=$(MINIO_CA_NAMESPACE) \
	CA_CERT=$(CA_CERT) \
	CA_KEY=$(CA_KEY) \
	$(MAKE) ca-secret

	CERT_SECRET_NAME=$(MINIO_CERT_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(MINIO_CERT_NAMESPACE) \
	CERT=$(MINIO_CERT) \
	KEY=$(MINIO_KEY) \
	$(MAKE) cert-secret-tls