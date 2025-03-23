##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GO ?= go
DOCKER ?= docker
KIND ?= $(LOCALBIN)/kind
KUBECTL ?= $(LOCALBIN)/kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GINKGO ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GORELEASER ?= $(LOCALBIN)/goreleaser
GO_LICENSES = $(LOCALBIN)/go-licenses
CRD_REF_DOCS = $(LOCALBIN)/crd-ref-docs
FLUX ?= $(LOCALBIN)/flux
YQ ?= $(LOCALBIN)/yq

## Tool Versions
KUBERNETES_VERSION ?= 1.32.x
KIND_VERSION ?= v0.27.0
KUBECTL_VERSION ?= v1.32.0
KUSTOMIZE_VERSION ?= v5.4.3
CONTROLLER_GEN_VERSION ?= v0.17.2
GINKGO_VERSION ?= v2.22.2
GOLANGCI_LINT_VERSION ?= v1.64.8
GORELEASER_VERSION ?= v2.8.1
GO_LICENSES_VERSION ?= v1.0.0
CRD_REF_DOCS_VERSION ?= v0.1.0
FLUX_VERSION ?= 0.40.1
JQ_VERSION ?= jq-1.7
YQ_VERSION ?= v4.18.1

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: kubectl
kubectl: ## Download kubectl locally if necessary.
ifeq (,$(wildcard $(KUBECTL)))
ifeq (,$(shell which kubectl 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(KUBECTL)) ;\
	OS=$(shell $(GO) env GOOS) && ARCH=$(shell $(GO) env GOARCH) && \
	curl -sSLo $(KUBECTL) https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/linux/$${ARCH}/kubectl ;\
	chmod +x $(KUBECTL) ;\
	}
else
KUBECTL = $(shell which kubectl)
endif
endif

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: goreleaser
goreleaser: $(GORELEASER) ## Download goreleaser locally if necessary.
$(GORELEASER): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

.PHONY: go-licenses
go-licenses: $(GO_LICENSES) ## Download go-licenses locally if necessary.
$(GO_LICENSES): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install github.com/google/go-licenses@$(GO_LICENSES_VERSION)

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: flux
flux: ## Download flux locally if necessary.
ifeq (,$(wildcard $(FLUX)))
ifeq (,$(shell which flux 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(FLUX)) ;\
	curl -sSLo - https://github.com/fluxcd/flux2/releases/download/v$(FLUX_VERSION)/flux_$(FLUX_VERSION)_linux_amd64.tar.gz| \
	tar xzf - -C bin/ ;\
	}
else
FLUX = $(shell which flux)
endif
endif

.PHONY: jq
jq: ## Download jq locally if necessary.
ifeq (,$(wildcard $(JQ)))
ifeq (,$(shell which jq 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(JQ)) ;\
	OS=$(shell $(GO) env GOOS) && ARCH=$(shell $(GO) env GOARCH) && \
	curl -sSLo $(JQ) https://github.com/jqlang/jq/releases/download/$(JQ_VERSION)/jq-linux-$${ARCH} ;\
	chmod +x $(JQ) ;\
	}
else
JQ = $(shell which jq)
endif
endif

.PHONY: yq
yq: ## Download yq locally if necessary.
ifeq (,$(wildcard $(YQ)))
ifeq (,$(shell which yq 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(YQ)) ;\
	curl -sSLo - https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_linux_amd64.tar.gz | \
	tar xzf - -C bin/ ;\
	mv bin/yq_linux_amd64 bin/yq ;\
	}
else
YQ = $(shell which yq)
endif
endif