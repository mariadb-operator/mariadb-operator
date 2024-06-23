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

WEBHOOK_PKI_DIR ?= /tmp/k8s-webhook-server/serving-certs
 
WEBHOOK_CERT ?= $(WEBHOOK_PKI_DIR)/tls.crt
WEBHOOK_KEY ?= $(WEBHOOK_PKI_DIR)/tls.key
WEBHOOK_CERT_SUBJECT ?= "/CN=localhost"
WEBHOOK_CERT_ALT_NAMES ?= "subjectAltName=DNS:localhost,IP:127.0.0.1"
.PHONY: cert-webhook
cert-webhook: ca ## Generates webhook private key and certificate for local development.
	PKI_DIR=$(WEBHOOK_PKI_DIR) \
	CA_CERT=$(CA_CERT) \
	CA_KEY=$(CA_KEY) \
	CERT=$(WEBHOOK_CERT) \
	KEY=$(WEBHOOK_KEY) \
	CERT_SUBJECT=$(WEBHOOK_CERT_SUBJECT) \
	CERT_ALT_NAMES=$(WEBHOOK_CERT_ALT_NAMES) \
	$(MAKE) cert

MINIO_PKI_DIR ?= /tmp/pki-minio

MINIO_CERT ?= $(MINIO_PKI_DIR)/tls.crt
MINIO_KEY ?= $(MINIO_PKI_DIR)/tls.key
MINIO_CERT_SUBJECT ?= "/CN=minio.minio.svc.cluster.local"
MINIO_CERT_ALT_NAMES ?= "subjectAltName=DNS:minio,DNS:minio.minio,DNS:minio.minio.svc.cluster.local"
MINIO_CERT_NAMESPACE ?= minio
.PHONY: cert-minio
cert-minio: ca kubectl ## Generates minio private key and certificate for local development.
	PKI_DIR=$(MINIO_PKI_DIR) \
	CA_CERT=$(CA_CERT) \
	CA_KEY=$(CA_KEY) \
	CERT=$(MINIO_CERT) \
	KEY=$(MINIO_KEY) \
	CERT_SUBJECT=$(MINIO_CERT_SUBJECT) \
	CERT_ALT_NAMES=$(MINIO_CERT_ALT_NAMES) \
	$(MAKE) cert
	$(KUBECTL) create namespace $(MINIO_CERT_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret tls minio-tls -n $(MINIO_CERT_NAMESPACE) \
		--cert=$(MINIO_CERT) --key=$(MINIO_KEY) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic minio-ca \
		--from-file=$(CA_CERT) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -