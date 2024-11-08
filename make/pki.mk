##@ PKI

PKI_DIR ?= /tmp/pki
CA_DIR ?= $(PKI_DIR)/ca
CERT_SIZE ?= 1024

.PHONY: ca
ca: ca-server ca-client ## Generates CA keypairs.

CA_SERVER_SECRET_NAME ?= mariadb-server-ca
CA_SERVER_SECRET_NAMESPACE ?= default
CA_SERVER_CERT ?= $(CA_DIR)/server.crt
CA_SERVER_KEY ?= $(CA_DIR)/server.key
CA_SERVER_SUBJECT ?= "/CN=mariadb-server-ca"
.PHONY: ca-server
ca-server: ## Generates server CA keypair.
	@if [ ! -f "$(CA_SERVER_CERT)" ] || [ ! -f "$(CA_SERVER_KEY)" ]; then \
		mkdir -p $(CA_DIR); \
		openssl req -new -newkey rsa:$(CERT_SIZE) -x509 -sha256 -days 365 -nodes \
			-out $(CA_SERVER_CERT) -keyout $(CA_SERVER_KEY) \
			-subj $(CA_SERVER_SUBJECT); \
	else \
		echo "CA server files already exist, skipping generation."; \
	fi

	CERT_SECRET_NAME=$(CA_SERVER_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(CA_SERVER_SECRET_NAMESPACE) \
	CERT=$(CA_SERVER_CERT) \
	KEY=$(CA_SERVER_KEY) \
	$(MAKE) cert-secret-tls

CA_CLIENT_SECRET_NAME ?= mariadb-client-ca
CA_CLIENT_SECRET_NAMESPACE ?= default
CA_CLIENT_CERT ?= $(CA_DIR)/client.crt
CA_CLIENT_KEY ?= $(CA_DIR)/client.key
CA_CLIENT_SUBJECT ?= "/CN=mariadb-client-ca"
.PHONY: ca-client
ca-client: ## Generates client CA keypair.
	@if [ ! -f "$(CA_CLIENT_CERT)" ] || [ ! -f "$(CA_CLIENT_KEY)" ]; then \
		mkdir -p $(CA_DIR); \
		openssl req -new -newkey rsa:$(CERT_SIZE) -x509 -sha256 -days 365 -nodes \
			-out $(CA_CLIENT_CERT) -keyout $(CA_CLIENT_KEY) \
			-subj $(CA_CLIENT_SUBJECT); \
	else \
		echo "CA client files already exist, skipping generation."; \
	fi

	CERT_SECRET_NAME=$(CA_CLIENT_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(CA_CLIENT_SECRET_NAMESPACE) \
	CERT=$(CA_CLIENT_CERT) \
	KEY=$(CA_CLIENT_KEY) \
	$(MAKE) cert-secret-tls

CERT_SECRET_NAME ?=
CERT_SECRET_NAMESPACE ?=
CA_CERT ?=
CA_KEY ?=
CERT ?=
KEY ?=
CERT_SUBJECT ?=
CERT_ALT_NAMES ?=

.PHONY: cert-leaf
cert-leaf: ## Generates leaf certificate keypair.
	@mkdir -p $(PKI_DIR)
	@openssl req -new -newkey rsa:$(CERT_SIZE) -x509 -sha256 -days 365 -nodes \
		-CA $(CA_CERT) -CAkey $(CA_KEY) \
		-out $(CERT) -keyout $(KEY) \
		-subj $(CERT_SUBJECT) -addext $(CERT_ALT_NAMES)

	CERT_SECRET_NAME=$(CERT_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(CERT_SECRET_NAMESPACE) \
	CERT=$(CERT) \
	KEY=$(KEY) \
	$(MAKE) cert-secret-tls

.PHONY: cert-leaf-client
cert-leaf-client: ## Generates leaf certificate keypair for a client.
	@mkdir -p $(PKI_DIR)
	@openssl req -new -newkey rsa:$(CERT_SIZE) -x509 -sha256 -days 365 -nodes \
		-CA $(CA_CERT) -CAkey $(CA_KEY) \
		-out $(CERT) -keyout $(KEY) \
		-subj $(CERT_SUBJECT)

	CERT_SECRET_NAME=$(CERT_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(CERT_SECRET_NAMESPACE) \
	CERT=$(CERT) \
	KEY=$(KEY) \
	$(MAKE) cert-secret-tls

MARIADB_NAME ?= mariadb
MARIADB_NAMESPACE ?= default
MARIADB_SERVER_CERT ?= $(PKI_DIR)/$(MARIADB_NAME)-server.crt
MARIADB_SERVER_KEY ?= $(PKI_DIR)/$(MARIADB_NAME)-server.key
MARIADB_CLIENT_CERT ?= $(PKI_DIR)/$(MARIADB_NAME)-client.crt
MARIADB_CLIENT_KEY ?= $(PKI_DIR)/$(MARIADB_NAME)-client.key
.PHONY: cert-leaf-mariadb
cert-leaf-mariadb: ca ## Generate leaf certificates for MariaDB.
	CERT_SECRET_NAME=$(MARIADB_NAME)-server-tls \
	CERT_SECRET_NAMESPACE=$(MARIADB_NAMESPACE) \
	CERT=$(MARIADB_SERVER_CERT) \
	KEY=$(MARIADB_SERVER_KEY) \
	CERT_SUBJECT="/CN=$(MARIADB_NAME).default.svc.cluster.local" \
	CERT_ALT_NAMES="$(CERT_ALT_NAMES)" \
	CA_CERT=$(CA_SERVER_CERT) \
	CA_KEY=$(CA_SERVER_KEY) \
	$(MAKE) cert-leaf

	CERT_SECRET_NAME=$(MARIADB_NAME)-client-tls \
	CERT_SECRET_NAMESPACE=$(MARIADB_NAMESPACE) \
	CERT=$(MARIADB_CLIENT_CERT) \
	KEY=$(MARIADB_CLIENT_KEY) \
	CERT_SUBJECT="/CN=$(MARIADB_NAME)-client" \
	CA_CERT=$(CA_CLIENT_CERT) \
	CA_KEY=$(CA_CLIENT_KEY) \
	$(MAKE) cert-leaf-client

.PHONY: cert-mariadb
cert-mariadb: ## Generate certificates for MariaDB.
	MARIADB_NAME="mariadb" \
	CERT_ALT_NAMES="subjectAltName=DNS:mariadb.default.svc.cluster.local,DNS:localhost" \
	$(MAKE) cert-leaf-mariadb

.PHONY: cert-mariadb-galera
cert-mariadb-galera: ## Generate certificates for MariaDB Galera.
	MARIADB_NAME="mariadb-galera" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.mariadb-galera-internal.default.svc.cluster.local,DNS:mariadb-galera-primary.default.svc.cluster.local,DNS:mariadb-galera.default.svc.cluster.local,DNS:localhost" \
	$(MAKE) cert-leaf-mariadb

WEBHOOK_PKI_DIR ?= /tmp/k8s-webhook-server/serving-certs
.PHONY: cert-webhook
cert-webhook: ca ## Generates webhook private key and certificate for local development.
	PKI_DIR=$(WEBHOOK_PKI_DIR) \
	CERT_SECRET_NAME=webhook \
	CERT_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_SERVER_CERT) \
	CA_KEY=$(CA_SERVER_KEY) \
	CERT=$(WEBHOOK_PKI_DIR)/tls.crt \
	KEY=$(WEBHOOK_PKI_DIR)/tls.key \
	CERT_SUBJECT="/CN=localhost" \
	CERT_ALT_NAMES="subjectAltName=DNS:localhost,IP:127.0.0.1" \
	$(MAKE) cert-leaf

MINIO_PKI_DIR ?= /tmp/pki-minio
MINIO_CA_SECRET_NAME ?= minio-ca
MINIO_CA_NAMESPACE ?= default
MINIO_CERT_SECRET_NAME ?= minio-tls
MINIO_CERT_NAMESPACE ?= minio
MINIO_CERT ?= $(MINIO_PKI_DIR)/tls.crt
MINIO_KEY ?= $(MINIO_PKI_DIR)/tls.key

.PHONY: cert-minio
cert-minio: ca kubectl ## Generates minio private key and certificate for local development.
	CERT_SECRET_NAME=$(MINIO_CA_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(MINIO_CA_NAMESPACE) \
	CERT=$(CA_SERVER_CERT) \
	KEY=$(CA_SERVER_KEY) \
	$(MAKE) cert-secret
	
	PKI_DIR=$(MINIO_PKI_DIR) \
	CERT_SECRET_NAME=$(MINIO_CERT_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(MINIO_CERT_NAMESPACE) \
	CA_CERT=$(CA_SERVER_CERT) \
	CA_KEY=$(CA_SERVER_KEY) \
	CERT=$(MINIO_CERT) \
	KEY=$(MINIO_KEY) \
	CERT_SUBJECT="/CN=minio.minio.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:minio,DNS:minio.minio,DNS:minio.minio.svc.cluster.local" \
	$(MAKE) cert-leaf

CERT_SECRET_NAME ?=
CERT_SECRET_NAMESPACE ?=
CERT ?=
KEY ?=

.PHONY: cert-secret-tls
cert-secret-tls: kubectl ## Creates a TLS Secret.
	$(KUBECTL) create namespace $(CERT_SECRET_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret tls $(CERT_SECRET_NAME) -n $(CERT_SECRET_NAMESPACE) \
		--cert=$(CERT) --key=$(KEY) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -

.PHONY: cert-secret
cert-secret: kubectl ## Creates a generic Secret from a certificate.
	$(KUBECTL) create namespace $(CERT_SECRET_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic $(CERT_SECRET_NAME) -n $(CERT_SECRET_NAMESPACE) \
		--from-file=$(CERT) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -

.PHONY: cert
cert: cert-mariadb cert-mariadb-galera cert-webhook cert-minio ## Generate certificates.