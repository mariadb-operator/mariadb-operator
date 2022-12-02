##@ Dev

CERTS_DIR=/tmp/k8s-webhook-server/serving-certs
CERTS_CONFIG=./hack/config/openssl.conf
certs: ## Generates development certificates.
	@mkdir -p ${CERTS_DIR}
	@openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -config ${CERTS_CONFIG} -out ${CERTS_DIR}/tls.crt -keyout ${CERTS_DIR}/tls.key

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=mariadb-manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-all
generate-all: generate manifests install ## Generate code and manifests.

.PHONY: lint
lint: golangci-lint ## Lint.
	$(GOLANGCI_LINT) run

.PHONY: test
test: manifests generate envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

.PHONY: cover
cover: test ## Run tests and generate coverage.
	@go tool cover -html=cover.out -o=cover.html