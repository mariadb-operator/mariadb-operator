##@ Dev

export MARIADB_OPERATOR_NAME ?= mariadb-operator
export MARIADB_OPERATOR_NAMESPACE ?= default
export MARIADB_OPERATOR_SA_PATH ?= /tmp/mariadb-operator/token

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

BACKUP_ENV ?= S3_ACCESS_KEY_ID=mariadb-operator S3_SECRET_ACCESS_KEY=Minio11!
BACKUP_COMMON_FLAGS ?= --path=backup --target-file-path=backup/0-backup-target.txt \
	--s3 --s3-bucket=backups --s3-endpoint=minio:9000 --s3-tls --s3-ca-cert-path=/tmp/certificate-authority/tls.crt \
	--log-dev --log-level=debug --log-time-encoder=iso8601 

BACKUP_FLAGS ?= --max-retention=8h $(BACKUP_COMMON_FLAGS)
.PHONY: backup
backup: lint ## Run backup from your host.
	$(BACKUP_ENV) go run cmd/controller/*.go backup $(BACKUP_FLAGS)

RESTORE_FLAGS ?= --target-time=1970-01-01T00:00:00Z $(BACKUP_COMMON_FLAGS)
.PHONY: restore
restore: lint ## Run restore from your host.
	$(BACKUP_ENV) go run cmd/controller/*.go backup restore $(RESTORE_FLAGS)
