##@ Dev

export MARIADB_OPERATOR_NAME ?= mariadb-operator
export MARIADB_OPERATOR_NAMESPACE ?= default
export MARIADB_OPERATOR_SA_PATH ?= /tmp/mariadb-operator/token

CERTS_DIR=/tmp/k8s-webhook-server/serving-certs
CERTS_CONFIG=./hack/config/openssl.conf
certs: ## Generates development certificates.
	@mkdir -p $(CERTS_DIR)
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -config $(CERTS_CONFIG) -out $(CERTS_DIR)/tls.crt -keyout $(CERTS_DIR)/tls.key

.PHONY: lint
lint: golangci-lint ## Lint.
	$(GOLANGCI_LINT) run

.PHONY: build
build: ## Build binary.
	go build -o bin/mariadb-operator cmd/main.go

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -timeout 20m -v ./... -coverprofile cover.out

.PHONY: cover
cover: test ## Run tests and generate coverage.
	@go tool cover -html=cover.out -o=cover.html

.PHONY: release
release: goreleaser ## Test release locally.
	$(GORELEASER) release --snapshot --rm-dist

##@ Operator

RUN_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601 --service-monitor-reconciler
.PHONY: run
run: lint ## Run a controller from your host.
	go run cmd/main.go $(RUN_FLAGS)

##@ Webhook

WEBHOOK_FLAGS ?= --log-dev
.PHONY: webhook
webhook: lint ## Run a webhook from your host.
	go run cmd/main.go webhook $(WEBHOOK_FLAGS)