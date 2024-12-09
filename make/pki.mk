##@ PKI

PKI_DIR ?= /tmp/pki
CA_DIR ?= $(PKI_DIR)/ca
CERT_SIZE ?= 4096

# CAs =====================================================================================================================================

.PHONY: ca
ca: ca-server ca-client ca-mxs-admin ca-minio ## Generates CA keypairs.

CA_SECRET_NAME ?=
CA_SECRET_NAMESPACE ?=
CA_CERT ?=
CA_KEY ?=
CA_SUBJECT ?= 
.PHONY: ca-root
ca-root: ## Generates a self-signed root CA keypair.
	@if [ ! -f "$(CA_CERT)" ] || [ ! -f "$(CA_KEY)" ]; then \
		mkdir -p $(CA_DIR); \
		openssl req -new -newkey rsa:$(CERT_SIZE) -x509 -sha256 -days 365 -nodes \
			-out $(CA_CERT) -keyout $(CA_KEY) \
			-subj $(CA_SUBJECT); \
	else \
		echo "CA files already exist, skipping generation."; \
	fi

	CERT_SECRET_NAME=$(CA_SECRET_NAME) \
	CERT_SECRET_NAMESPACE=$(CA_SECRET_NAMESPACE) \
	CERT=$(CA_CERT) \
	KEY=$(CA_KEY) \
	$(MAKE) cert-secret-tls

CA_SERVER_CERT ?= $(CA_DIR)/server.crt 
CA_SERVER_KEY ?= $(CA_DIR)/server.key 
.PHONY: ca-server
ca-server: ## Generates server CA keypair.
	CA_SECRET_NAME=mariadb-server-ca \
	CA_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_SERVER_CERT) \
	CA_KEY=$(CA_SERVER_KEY) \
	CA_SUBJECT="/CN=mariadb-server-ca" \
	$(MAKE) ca-root

CA_CLIENT_CERT ?= $(CA_DIR)/client.crt 
CA_CLIENT_KEY ?= $(CA_DIR)/client.key 
.PHONY: ca-client
ca-client: ## Generates client CA keypair.
	CA_SECRET_NAME=mariadb-client-ca \
	CA_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_CLIENT_CERT) \
	CA_KEY=$(CA_CLIENT_KEY) \
	CA_SUBJECT="/CN=mariadb-client-ca" \
	$(MAKE) ca-root

CA_MAXSCALE_ADMIN_CERT ?= $(CA_DIR)/maxscale-admin.crt 
CA_MAXSCALE_ADMIN_KEY ?= $(CA_DIR)/maxscale-admin.key 
.PHONY: ca-mxs-admin
ca-mxs-admin: ## Generates MaxScale admin CA keypair.
	CA_SECRET_NAME=maxscale-admin-ca \
	CA_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_MAXSCALE_ADMIN_CERT) \
	CA_KEY=$(CA_MAXSCALE_ADMIN_KEY) \
	CA_SUBJECT="/CN=maxscale-admin-ca" \
	$(MAKE) ca-root

CA_MAXSCALE_LISTENER_CERT ?= $(CA_DIR)/maxscale-listener.crt 
CA_MAXSCALE_LISTENER_KEY ?= $(CA_DIR)/maxscale-listener.key 
.PHONY: ca-mxs-listener
ca-mxs-listener: ## Generates MaxScale listener CA keypair.
	CA_SECRET_NAME=maxscale-listener-ca \
	CA_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_MAXSCALE_LISTENER_CERT) \
	CA_KEY=$(CA_MAXSCALE_LISTENER_KEY) \
	CA_SUBJECT="/CN=maxscale-listener-ca" \
	$(MAKE) ca-root

CA_MINIO_CERT ?= $(CA_DIR)/minio.crt 
CA_MINIO_KEY ?= $(CA_DIR)/minio.key 
.PHONY: ca-minio
ca-minio: ## Generates minio CA keypair.
	CA_SECRET_NAME=minio-ca \
	CA_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_MINIO_CERT) \
	CA_KEY=$(CA_MINIO_KEY) \
	CA_SUBJECT="/CN=minio-ca" \
	$(MAKE) ca-root

CERT_SECRET_NAME ?=
CERT_SECRET_NAMESPACE ?=
CA_CERT ?=
CA_KEY ?=
CERT ?=
KEY ?=
CERT_SUBJECT ?=
CERT_ALT_NAMES ?=

# Leaf certs ==============================================================================================================================

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

# MariaDB =================================================================================================================================

MARIADB_NAME ?= mariadb
MARIADB_NAMESPACE ?= default

.PHONY: cert-leaf-mariadb
cert-leaf-mariadb: cert-leaf-mariadb-server cert-leaf-mariadb-client ## Generate leaf certificates for MariaDB.

.PHONY: cert-leaf-mariadb-server
cert-leaf-mariadb-server: ca-server ## Generate server leaf certificate for MariaDB.
	CERT_SECRET_NAME=$(MARIADB_NAME)-server-tls \
	CERT_SECRET_NAMESPACE=$(MARIADB_NAMESPACE) \
	CERT=$(PKI_DIR)/$(MARIADB_NAME)-server.crt \
	KEY=$(PKI_DIR)/$(MARIADB_NAME)-server.key \
	CERT_SUBJECT="/CN=$(MARIADB_NAME).default.svc.cluster.local" \
	CERT_ALT_NAMES="$(CERT_ALT_NAMES)" \
	CA_CERT=$(CA_SERVER_CERT) \
	CA_KEY=$(CA_SERVER_KEY) \
	$(MAKE) cert-leaf

.PHONY: cert-leaf-mariadb-client
cert-leaf-mariadb-client: ca-client ## Generate client leaf certificate for MariaDB.
	CERT_SECRET_NAME=$(MARIADB_NAME)-client-tls \
	CERT_SECRET_NAMESPACE=$(MARIADB_NAMESPACE) \
	CERT=$(PKI_DIR)/$(MARIADB_NAME)-client.crt \
	KEY=$(PKI_DIR)/$(MARIADB_NAME)-client.key \
	CERT_SUBJECT="/CN=$(MARIADB_NAME)-client" \
	CA_CERT=$(CA_CLIENT_CERT) \
	CA_KEY=$(CA_CLIENT_KEY) \
	$(MAKE) cert-leaf-client

.PHONY: cert-mariadb
cert-mariadb: ## Generate certificates for MariaDB.
	MARIADB_NAME="mariadb" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.mariadb-internal.default.svc.cluster.local,DNS:*.mariadb-internal,DNS:mariadb.default.svc.cluster.local,DNS:localhost" \
	$(MAKE) cert-leaf-mariadb

.PHONY: cert-mariadb-galera
cert-mariadb-galera: ## Generate certificates for MariaDB Galera.
	MARIADB_NAME="mariadb-galera" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.mariadb-galera-internal.default.svc.cluster.local,DNS:*.mariadb-galera-internal,DNS:mariadb-galera-primary.default.svc.cluster.local,DNS:mariadb-galera.default.svc.cluster.local,DNS:localhost" \
	$(MAKE) cert-leaf-mariadb

.PHONY: cert-mariadb-repl
cert-mariadb-repl: ## Generate certificates for MariaDB replication.
	MARIADB_NAME="mariadb-repl" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.mariadb-repl-internal.default.svc.cluster.local,DNS:*.mariadb-repl-internal,DNS:mariadb-repl-primary.default.svc.cluster.local,DNS:mariadb-repl.default.svc.cluster.local,DNS:localhost" \
	$(MAKE) cert-leaf-mariadb

# MaxScale ================================================================================================================================

.PHONY: cert-mxs-galera
cert-mxs-galera: cert-mxs-galera-admin cert-mxs-galera-listener ## Generate certificates for MaxScale Galera.

.PHONY: cert-mxs-galera-admin
cert-mxs-galera-admin: ca-mxs-admin ## Generate admin certificates for MaxScale Galera.
	CERT_SECRET_NAME=maxscale-galera-admin-tls \
	CERT_SECRET_NAMESPACE=default \
	CERT=$(PKI_DIR)/maxscale-galera-admin.crt \
	KEY=$(PKI_DIR)/maxscale-galera-admin.key \
	CERT_SUBJECT="/CN=maxscale-galera.default.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.maxscale-galera-internal.default.svc.cluster.local,DNS:maxscale-galera.default.svc.cluster.local,DNS:maxscale-galera-gui.default.svc.cluster.local"  \
	CA_CERT=$(CA_MAXSCALE_ADMIN_CERT) \
	CA_KEY=$(CA_MAXSCALE_ADMIN_KEY) \
	$(MAKE) cert-leaf

.PHONY: cert-mxs-galera-listener
cert-mxs-galera-listener: ca-mxs-listener ## Generate listener certificates for MaxScale Galera.
	CERT_SECRET_NAME=maxscale-galera-listener-tls \
	CERT_SECRET_NAMESPACE=default \
	CERT=$(PKI_DIR)/maxscale-galera-listener.crt \
	KEY=$(PKI_DIR)/maxscale-galera-listener.key \
	CERT_SUBJECT="/CN=maxscale-galera.default.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.maxscale-galera-internal.default.svc.cluster.local,DNS:maxscale-galera.default.svc.cluster.local,DNS:maxscale-galera-gui.default.svc.cluster.local"  \
	CA_CERT=$(CA_MAXSCALE_LISTENER_CERT) \
	CA_KEY=$(CA_MAXSCALE_LISTENER_KEY) \
	$(MAKE) cert-leaf

.PHONY: cert-mxs-repl
cert-mxs-repl: cert-mxs-repl-admin cert-mxs-repl-listener ## Generate certificates for MaxScale replication.

.PHONY: cert-mxs-repl-admin
cert-mxs-repl-admin: ca-mxs-admin ## Generate admin certificates for MaxScale replication.
	CERT_SECRET_NAME=maxscale-repl-admin-tls \
	CERT_SECRET_NAMESPACE=default \
	CERT=$(PKI_DIR)/maxscale-repl-admin.crt \
	KEY=$(PKI_DIR)/maxscale-repl-admin.key \
	CERT_SUBJECT="/CN=maxscale-repl.default.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.maxscale-repl-internal.default.svc.cluster.local,DNS:maxscale-repl.default.svc.cluster.local,DNS:maxscale-repl-gui.default.svc.cluster.local"  \
	CA_CERT=$(CA_MAXSCALE_ADMIN_CERT) \
	CA_KEY=$(CA_MAXSCALE_ADMIN_KEY) \
	$(MAKE) cert-leaf

.PHONY: cert-mxs-repl-listener
cert-mxs-repl-listener: ca-mxs-listener ## Generate listener certificates for MaxScale replication.
	CERT_SECRET_NAME=maxscale-repl-listener-tls \
	CERT_SECRET_NAMESPACE=default \
	CERT=$(PKI_DIR)/maxscale-repl-listener.crt \
	KEY=$(PKI_DIR)/maxscale-repl-listener.key \
	CERT_SUBJECT="/CN=maxscale-repl.default.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:*.maxscale-repl-internal.default.svc.cluster.local,DNS:maxscale-repl.default.svc.cluster.local,DNS:maxscale-repl-gui.default.svc.cluster.local"  \
	CA_CERT=$(CA_MAXSCALE_LISTENER_CERT) \
	CA_KEY=$(CA_MAXSCALE_LISTENER_KEY) \
	$(MAKE) cert-leaf

# Webhook =================================================================================================================================

WEBHOOK_PKI_DIR ?= /tmp/k8s-webhook-server/serving-certs
.PHONY: cert-webhook
cert-webhook: ca ## Generates webhook private key and certificate for local development.
	PKI_DIR=$(WEBHOOK_PKI_DIR) \
	CERT_SECRET_NAME=webhook-tls \
	CERT_SECRET_NAMESPACE=default \
	CA_CERT=$(CA_SERVER_CERT) \
	CA_KEY=$(CA_SERVER_KEY) \
	CERT=$(WEBHOOK_PKI_DIR)/tls.crt \
	KEY=$(WEBHOOK_PKI_DIR)/tls.key \
	CERT_SUBJECT="/CN=localhost" \
	CERT_ALT_NAMES="subjectAltName=DNS:localhost,IP:127.0.0.1" \
	$(MAKE) cert-leaf

# Minio ===================================================================================================================================

MINIO_PKI_DIR ?= /tmp/pki-minio
.PHONY: cert-minio
cert-minio: ca ## Generates minio private key and certificate for local development.
	PKI_DIR=$(MINIO_PKI_DIR) \
	CERT_SECRET_NAME=minio-tls \
	CERT_SECRET_NAMESPACE=minio \
	CA_CERT=$(CA_MINIO_CERT) \
	CA_KEY=$(CA_MINIO_KEY) \
	CERT=$(MINIO_PKI_DIR)/tls.crt \
	KEY=$(MINIO_PKI_DIR)/tls.key \
	CERT_SUBJECT="/CN=minio.minio.svc.cluster.local" \
	CERT_ALT_NAMES="subjectAltName=DNS:minio,DNS:minio.minio,DNS:minio.minio.svc.cluster.local" \
	$(MAKE) cert-leaf

# Secrets =================================================================================================================================

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
	$(KUBECTL) label secret $(CERT_SECRET_NAME) -n $(CERT_SECRET_NAMESPACE) \
		k8s.mariadb.com/watch=""

.PHONY: cert-secret
cert-secret: kubectl ## Creates a generic Secret from a certificate.
	$(KUBECTL) create namespace $(CERT_SECRET_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic $(CERT_SECRET_NAME) -n $(CERT_SECRET_NAMESPACE) \
		--from-file=$(CERT) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) label secret $(CERT_SECRET_NAME) -n $(CERT_SECRET_NAMESPACE) \
		k8s.mariadb.com/watch=""

# Entrypoint ==============================================================================================================================

.PHONY: cert
cert: cert-mariadb cert-mariadb-galera cert-mariadb-repl cert-mxs-galera cert-mxs-repl cert-webhook cert-minio ## Generate certificates.