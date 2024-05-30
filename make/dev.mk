MARIADB_OPERATOR_NAME ?= mariadb-operator
MARIADB_OPERATOR_NAMESPACE ?= default
MARIADB_OPERATOR_SA_PATH ?= /tmp/mariadb-operator/token
WATCH_NAMESPACE ?= ""

ENV ?= \
	RELATED_IMAGE_MARIADB=$(RELATED_IMAGE_MARIADB) \
	RELATED_IMAGE_MAXSCALE=$(RELATED_IMAGE_MAXSCALE) \
	RELATED_IMAGE_EXPORTER=$(RELATED_IMAGE_EXPORTER) \
	RELATED_IMAGE_EXPORTER_MAXSCALE=$(RELATED_IMAGE_EXPORTER_MAXSCALE) \
	MARIADB_GALERA_INIT_IMAGE=$(MARIADB_GALERA_INIT_IMAGE) \
	MARIADB_GALERA_AGENT_IMAGE=$(MARIADB_GALERA_AGENT_IMAGE) \
	MARIADB_GALERA_LIB_PATH=$(MARIADB_GALERA_LIB_PATH) \
	MARIADB_OPERATOR_IMAGE=$(IMG) \
	MARIADB_OPERATOR_NAME=$(MARIADB_OPERATOR_NAME) \
	MARIADB_OPERATOR_NAMESPACE=$(MARIADB_OPERATOR_NAMESPACE) \
	MARIADB_OPERATOR_SA_PATH=$(MARIADB_OPERATOR_SA_PATH) \
	WATCH_NAMESPACE=$(WATCH_NAMESPACE)

ENV_ENT ?= \
	RELATED_IMAGE_MARIADB=$(RELATED_IMAGE_MARIADB_ENT) \
	RELATED_IMAGE_MAXSCALE=$(RELATED_IMAGE_MAXSCALE_ENT) \
	RELATED_IMAGE_EXPORTER=$(RELATED_IMAGE_EXPORTER_ENT) \
	RELATED_IMAGE_EXPORTER_MAXSCALE=$(RELATED_IMAGE_EXPORTER_MAXSCALE_ENT) \
	MARIADB_GALERA_INIT_IMAGE=$(MARIADB_GALERA_INIT_IMAGE_ENT) \
	MARIADB_GALERA_AGENT_IMAGE=$(MARIADB_GALERA_AGENT_IMAGE_ENT) \
	MARIADB_GALERA_LIB_PATH=$(MARIADB_GALERA_LIB_PATH_ENT) \
	MARIADB_OPERATOR_IMAGE=$(IMG_ENT) \
	MARIADB_OPERATOR_NAME=$(MARIADB_OPERATOR_NAME) \
	MARIADB_OPERATOR_NAMESPACE=$(MARIADB_OPERATOR_NAMESPACE) \
	MARIADB_OPERATOR_SA_PATH=$(MARIADB_OPERATOR_SA_PATH) \
	WATCH_NAMESPACE=$(WATCH_NAMESPACE) \
	ENTERPRISE=true

TEST_ARGS ?= --timeout 20m -procs 1

TEST_ENV ?= $(ENV) KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"
TEST ?= $(TEST_ENV) $(GINKGO) -p --coverprofile=cover.out --label-filter='!enterprise' $(TEST_ARGS)

TEST_ENV_ENT ?= $(ENV_ENT) KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"
TEST_ENT ?= $(TEST_ENV_ENT) $(GINKGO) -p --coverprofile=cover.out $(TEST_ARGS)

GOCOVERDIR ?= .

##@ Test

.PHONY: test-unit
test-unit: envtest ginkgo ## Run unit tests.
	$(TEST) ./pkg/... ./api/...

.PHONY: test-int
test-int: envtest ginkgo ## Run integration tests.
	$(TEST) ./controller/...

.PHONY: test
test: test-unit test-int ## Run tests.

.PHONY: cover
cover: ## Generate and view coverage report.
	@go tool cover -html=cover.out -o=cover.html
	open cover.html

##@ Test Enterprise

.PHONY: test-unit-ent
test-unit-ent: envtest ginkgo ## Run enterprise unit tests.
	$(TEST_ENT) ./pkg/... ./api/...

.PHONY: test-int-ent
test-int-ent: envtest ginkgo ## Run enterprise integration tests.
	$(TEST_ENT) ./controller/...

.PHONY: test-ent
test-ent: test-unit-ent test-int-ent ## Run enterprise tests.

##@ Lint

.PHONY: lint
lint: golangci-lint ## Lint.
	$(GOLANGCI_LINT) run

##@ Release

.PHONY: release
release: goreleaser ## Test release locally.
	$(GORELEASER) release --snapshot --rm-dist

##@ Run

RUN_FLAGS ?= --log-dev --log-level=info --log-time-encoder=iso8601

.PHONY: run
run: lint ## Run a controller from your host.
	$(ENV) go run cmd/controller/*.go $(RUN_FLAGS)

.PHONY: run-ent
run-ent: lint cert ## Run a enterprise controllers from your host.
	$(ENV_ENT) go run cmd/enterprise/*.go $(RUN_FLAGS)

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

BACKUP_ENV ?= AWS_ACCESS_KEY_ID=mariadb-operator AWS_SECRET_ACCESS_KEY=Minio11!
BACKUP_COMMON_FLAGS ?= --path=backup --target-file-path=backup/0-backup-target.txt \
	--s3 --s3-bucket=backups --s3-endpoint=minio:9000 --s3-region=us-east-1 --s3-tls --s3-ca-cert-path=/tmp/certificate-authority/tls.crt \
	--log-dev --log-level=debug --log-time-encoder=iso8601

BACKUP_FLAGS ?= --max-retention=8h $(BACKUP_COMMON_FLAGS)
.PHONY: backup
backup: lint ## Run backup from your host.
	$(BACKUP_ENV) go run cmd/controller/*.go backup $(BACKUP_FLAGS)

RESTORE_FLAGS ?= --target-time=1970-01-01T00:00:00Z $(BACKUP_COMMON_FLAGS)
.PHONY: restore
restore: lint ## Run restore from your host.
	$(BACKUP_ENV) go run cmd/controller/*.go backup restore $(RESTORE_FLAGS)

.PHONY: init-dir
init-dir: ## Create config and state directories for init local development.
	mkdir -p mariadb/config
	mkdir -p mariadb/state

INIT_ENV ?= KUBECONFIG=$(HOME)/.kube/config POD_NAME=mariadb-galera-0 MARIADB_ROOT_PASSWORD=MariaDB11!
INIT_FLAGS ?= $(RUN_FLAGS) --mariadb-name=mariadb-galera --mariadb-namespace=default --config-dir=mariadb/config --state-dir=mariadb/state
.PHONY: init
init: init-dir ## Run init from your host.
	$(INIT_ENV) go run cmd/controller/*.go init $(INIT_FLAGS)

AGENT_ENV ?= KUBECONFIG=$(HOME)/.kube/config
# AGENT_AUTH_FLAGS ?= --kubernetes-auth=true --kubernetes-trusted-name=mariadb-galera --kubernetes-trusted-namespace=default
AGENT_AUTH_FLAGS ?=
AGENT_FLAGS ?= $(RUN_FLAGS) $(AGENT_AUTH_FLAGS) --config-dir=mariadb/config --state-dir=mariadb/state
.PHONY: agent
agent: ## Run agent from your host.
	$(AGENT_ENV) go run cmd/controller/*.go agent $(AGENT_FLAGS)