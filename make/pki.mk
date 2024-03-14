##@ PKI

CA_DIR ?= /tmp/certificate-authority
CA_CERT ?= $(CA_DIR)/tls.crt
CA_KEY ?= $(CA_DIR)/tls.key
.PHONY: ca
ca: ## Generates CA private key and certificate for local development.
	@if [ ! -f "$(CA_CERT)" ] || [ ! -f "$(CA_KEY)" ]; then \
		mkdir -p $(CA_DIR); \
		openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes \
			-subj "/CN=mariadb-operator" -out $(CA_CERT) -keyout $(CA_KEY); \
	else \
		echo "CA files already exist, skipping generation."; \
	fi

CERT_DIR ?= /tmp/k8s-webhook-server/serving-certs
CERT_SUBJECT ?= "/CN=localhost"
CERT_ALT_NAMES ?= "subjectAltName=DNS:localhost,IP:127.0.0.1"
.PHONY: cert
cert: ca ## Generates webhook private key and certificate for local development.
	@mkdir -p $(CERT_DIR)
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes \
		-subj $(CERT_SUBJECT) -addext $(CERT_ALT_NAMES) \
		-out $(CERT_DIR)/tls.crt -keyout $(CERT_DIR)/tls.key \
		-CA $(CA_CERT) -CAkey $(CA_KEY)

MINIO_CERT_DIR ?= /tmp/minio-certs
MINIO_CERT_SUBJECT ?= "/CN=minio.minio.svc.cluster.local"
MINIO_CERT_ALT_NAMES ?= "subjectAltName=DNS:minio,DNS:minio.minio,DNS:minio.minio.svc.cluster.local"
MINIO_CERT_NAMESPACE ?= minio
.PHONY: cert-minio
cert-minio: ca kubectl ## Generates minio private key and certificate for local development.
	CERT_DIR=$(MINIO_CERT_DIR) CERT_SUBJECT=$(MINIO_CERT_SUBJECT) CERT_ALT_NAMES=$(MINIO_CERT_ALT_NAMES) $(MAKE) cert
	$(KUBECTL) create namespace $(MINIO_CERT_NAMESPACE) \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret tls minio-tls -n $(MINIO_CERT_NAMESPACE) \
		--cert=$(MINIO_CERT_DIR)/tls.crt --key=$(MINIO_CERT_DIR)/tls.key \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) create secret generic minio-ca \
		--from-file=ca.crt=$(CA_DIR)/tls.crt \
		--dry-run=client -o yaml | $(KUBECTL) apply -f -