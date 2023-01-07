##@ Dev

CERTS_DIR=/tmp/k8s-webhook-server/serving-certs
CERTS_CONFIG=./hack/config/openssl.conf
certs: ## Generates development certificates.
	@mkdir -p ${CERTS_DIR}
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -config ${CERTS_CONFIG} -out ${CERTS_DIR}/tls.crt -keyout ${CERTS_DIR}/tls.key

.PHONY: lint
lint: golangci-lint ## Lint.
	$(GOLANGCI_LINT) run

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: cover
cover: test ## Run tests and generate coverage.
	@go tool cover -html=cover.out -o=cover.html

.PHONY: release
release: goreleaser ## Test release locally.
	$(GORELEASER) release --snapshot --rm-dist