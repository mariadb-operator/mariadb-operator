##@ Dev

export MARIADB_OPERATOR_NAME ?= mariadb-operator
export MARIADB_OPERATOR_NAMESPACE ?= default
export MARIADB_OPERATOR_SA_PATH ?= /tmp/mariadb-operator/token

CA_DIR=/tmp/k8s-webhook-server/certificate-authority
CERT_DIR=/tmp/k8s-webhook-server/serving-certs
CA_CONFIG=./hack/config/openssl_ca.conf
CERT_CONFIG=./hack/config/openssl_cert.conf

.PHONY: ca
ca: ## Generates CA private key and certificate for local development.
	@mkdir -p $(CA_DIR)
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes \
		-config $(CA_CONFIG) -out $(CA_DIR)/tls.crt -keyout $(CA_DIR)/tls.key

.PHONY: cert
cert: ca ## Generates webhook private key and certificate for local development.
	@mkdir -p $(CERT_DIR)
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes \
		-config $(CERT_CONFIG) -out $(CERT_DIR)/tls.crt -keyout $(CERT_DIR)/tls.key \
		-CA $(CA_DIR)/tls.crt -CAkey $(CA_DIR)/tls.key

.PHONY: lint
lint: golangci-lint ## Lint.
	$(GOLANGCI_LINT) run

TEST_ENV ?= RELATED_IMAGE_MARIADB=$(RELATED_IMAGE_MARIADB) RELATED_IMAGE_EXPORTER=$(RELATED_IMAGE_EXPORTER) MARIADB_OPERATOR_IMAGE=$(IMG) \
	MARIADB_OPERATOR_NAME=$(MARIADB_OPERATOR_NAME) MARIADB_OPERATOR_NAMESPACE=$(MARIADB_OPERATOR_NAMESPACE) MARIADB_OPERATOR_SA_PATH=$(MARIADB_OPERATOR_SA_PATH) \
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"
.PHONY: test
test: envtest ## Run tests.
	 $(TEST_ENV) go test -timeout 20m -v ./... -coverprofile cover.out

.PHONY: cover
cover: test ## Run tests and generate coverage.
	@go tool cover -html=cover.out -o=cover.html

.PHONY: release
release: goreleaser ## Test release locally.
	$(GORELEASER) release --snapshot --rm-dist

##@ Run

WATCH_NAMESPACE ?= ""
RUN_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601

RUN_ENV ?= RELATED_IMAGE_MARIADB=$(RELATED_IMAGE_MARIADB) RELATED_IMAGE_EXPORTER=$(RELATED_IMAGE_EXPORTER) MARIADB_OPERATOR_IMAGE=$(IMG) WATCH_NAMESPACE=$(WATCH_NAMESPACE)
.PHONY: run
run: lint ## Run a controller from your host.
	$(RUN_ENV) go run cmd/controller/*.go $(RUN_FLAGS)

RUN_ENT_ENV ?= RELATED_IMAGE_MARIADB=$(RELATED_IMAGE_MARIADB_ENT) RELATED_IMAGE_EXPORTER=$(RELATED_IMAGE_EXPORTER) MARIADB_OPERATOR_IMAGE=$(IMG_ENT) WATCH_NAMESPACE=$(WATCH_NAMESPACE)
.PHONY: run-ent
run-ent: lint cert ## Run a enterprise from your host.
	$(RUN_ENT_ENV) go run cmd/enterprise/*.go $(RUN_FLAGS)

WEBHOOK_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601 \
	--ca-cert-path=$(CA_DIR)/tls.crt --cert-dir=$(CERT_DIR) \
	--validate-cert=false
.PHONY: webhook
webhook: lint cert ## Run a webhook from your host.
	go run cmd/controller/*.go webhook $(WEBHOOK_FLAGS)

CERT_CONTROLLER_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601 \
	--ca-validity=24h --cert-validity=1h --lookahead-validity=8h --requeue-duration=1m
.PHONY: cert-controller
cert-controller: lint ## Run a cert-controller from your host.
	go run cmd/controller/*.go cert-controller $(CERT_CONTROLLER_FLAGS)

BACKUP_FLAGS ?= --path=backup --max-retention=1h --target-file-path=backup/0-backup-target.txt \
	--log-dev --log-level=debug --log-time-encoder=iso8601 
.PHONY: backup
backup: lint ## Run backup from your host.
	go run cmd/controller/*.go backup $(BACKUP_FLAGS)

RESTORE_FLAGS ?= --path=backup --target-time=1970-01-01T00:00:00Z --target-file-path=backup/0-backup-target.txt \
	--log-dev --log-level=debug --log-time-encoder=iso8601 
.PHONY: backup-restore
backup-restore: lint ## Run restore from your host.
	go run cmd/controller/*.go backup restore $(RESTORE_FLAGS)
