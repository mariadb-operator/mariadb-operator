##@ Dev

export MARIADB_OPERATOR_NAME ?= mariadb-operator
export MARIADB_OPERATOR_NAMESPACE ?= default
export MARIADB_OPERATOR_SA_PATH ?= /tmp/mariadb-operator/token

CA_DIR=/tmp/k8s-webhook-server/certificate-authority
CA_SECRET=mariadb-operator-webhook-ca 
CERT_DIR=/tmp/k8s-webhook-server/serving-certs
CERT_SECRET=mariadb-operator-webhook-cert
CERT_CONFIG=./hack/config/openssl.conf

RUN_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601 --service-monitor-reconciler

.PHONY: cert
cert: ## Generates development certificate.
	@mkdir -p $(CERT_DIR)
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -config $(CERT_CONFIG) -out $(CERT_DIR)/tls.crt -keyout $(CERT_DIR)/tls.key

.PHONY: cert-from-cluster
cert-from-cluster: ## Get certificate from cluster.
	@mkdir -p $(CA_DIR)
	@mkdir -p $(CERT_DIR)
	@kubectl get secret -o json $(CA_SECRET) | jq -r ".data.\"tls.crt\"" | base64 -d > $(CA_DIR)/tls.crt
	@kubectl get secret -o json $(CA_SECRET) | jq -r ".data.\"tls.key\"" | base64 -d > $(CA_DIR)/tls.key
	@kubectl get secret -o json $(CERT_SECRET) | jq -r ".data.\"tls.crt\"" | base64 -d > $(CERT_DIR)/tls.crt
	@kubectl get secret -o json $(CERT_SECRET) | jq -r ".data.\"tls.key\"" | base64 -d > $(CERT_DIR)/tls.key

.PHONY: lint
lint: golangci-lint ## Lint.
	$(GOLANGCI_LINT) run

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test -timeout 20m -v ./... -coverprofile cover.out

.PHONY: cover
cover: test ## Run tests and generate coverage.
	@go tool cover -html=cover.out -o=cover.html

.PHONY: release
release: goreleaser ## Test release locally.
	$(GORELEASER) release --snapshot --rm-dist

##@ Run

.PHONY: run
run: lint ## Run a controller from your host.
	go run cmd/controller/*.go $(RUN_FLAGS)

WEBHOOK_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601 
.PHONY: webhook
webhook: lint cert-from-cluster ## Run a webhook from your host.
	go run cmd/controller/*.go webhook $(WEBHOOK_FLAGS)

CERT_CONTROLLER_FLAGS ?= --log-dev --log-level=debug --log-time-encoder=iso8601 --ca-validity=24h --cert-validity=1h --lookahead-validity=8h --requeue-duration=1m
.PHONY: cert-controller
cert-controller: lint ## Run a cert-controller from your host.
	go run cmd/controller/*.go cert-controller $(CERT_CONTROLLER_FLAGS)

.PHONY: run-ent
run-ent: lint cert ## Run a enterprise from your host.
	go run cmd/enterprise/*.go $(RUN_FLAGS)