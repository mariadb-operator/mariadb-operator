##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
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
MDTOC = $(LOCALBIN)/mdtoc
FLUX ?= $(LOCALBIN)/flux
YQ ?= $(LOCALBIN)/yq
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
OPM ?= $(LOCALBIN)/opm
OC ?= $(LOCALBIN)/oc
PREFLIGHT ?= $(LOCALBIN)/preflight

## Tool Versions
KIND_VERSION ?= v0.22.0
KUBECTL_VERSION ?= v1.29.3
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_GEN_VERSION ?= v0.14.0
ENVTEST_K8S_VERSION ?= 1.29
GINKGO_VERSION ?= v2.17.2
GOLANGCI_LINT_VERSION ?= v1.56.2
GORELEASER_VERSION ?= v1.24.0
GO_LICENSES_VERSION ?= v1.0.0
CRD_REF_DOCS_VERSION ?= v0.0.11
MDTOC_VERSION ?= v1.3.0
FLUX_VERSION ?= 0.40.1
JQ_VERSION ?= jq-1.7
YQ_VERSION ?= v4.18.1
OPERATOR_SDK_VERSION ?= v1.34.1
OPM_VERSION ?= v1.37.0
OC_TAR_GZ_URL ?= https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/latest-4.9/openshift-client-linux-4.9.59.tar.gz
PREFLIGHT_VERSION ?= 1.9.7

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: kubectl
kubectl: ## Download kubectl locally if necessary.
ifeq (,$(wildcard $(KUBECTL)))
ifeq (,$(shell which kubectl 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(KUBECTL)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
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
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: goreleaser
goreleaser: $(GORELEASER) ## Download goreleaser locally if necessary.
$(GORELEASER): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)

.PHONY: go-licenses
go-licenses: $(GO_LICENSES) ## Download go-licenses locally if necessary.
$(GO_LICENSES): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/google/go-licenses@$(GO_LICENSES_VERSION)

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: mdtoc
mdtoc: $(MDTOC) ## Download mdtoc locally if necessary.
$(MDTOC): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/mdtoc@$(MDTOC_VERSION)

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
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
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

.PHONY: operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

.PHONY: oc
oc: ## Download oc locally if necessary.
ifeq (,$(wildcard $(OC)))
ifeq (,$(shell which oc 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OC)) ;\
	curl -sSLo $(OC).tar.gz $(OC_TAR_GZ_URL) ;\
	tar -C $(LOCALBIN) -zxvf $(OC).tar.gz ;\
	chmod +x $(OC) ;\
	rm $(OC).tar.gz ;\
	}
else
OC = $(shell which oc)
endif
endif

.PHONY: preflight
preflight: ## Download preflight locally if necessary.
ifeq (,$(wildcard $(PREFLIGHT)))
ifeq (,$(shell which preflight 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(PREFLIGHT)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(PREFLIGHT) https://github.com/redhat-openshift-ecosystem/openshift-preflight/releases/download/$(PREFLIGHT_VERSION)/preflight-$${OS}-$${ARCH} ;\
	chmod +x $(PREFLIGHT) ;\
	}
else
PREFLIGHT = $(shell which preflight)
endif
endif