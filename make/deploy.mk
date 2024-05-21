HELM_DIR ?= deploy/charts/mariadb-operator
CLUSTER ?= mdb

##@ Cluster

KIND_CONFIG ?= hack/config/kind.yaml
KIND_IMAGE ?= kindest/node:v1.29.2

.PHONY: cluster
cluster: kind ## Create a single node kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --image $(KIND_IMAGE)

.PHONY: cluster-ha
cluster-ha: kind ## Create a HA kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

.PHONY: cluster-delete
cluster-delete: kind ## Delete the kind cluster.
	$(KIND) delete cluster --name $(CLUSTER)

.PHONY: cluster-ctx
cluster-ctx: kubectl ## Sets cluster context.
	$(KUBECTL) config use-context kind-$(CLUSTER)

.PHONY: cluster-ls
cluster-ps: ## List all cluster Nodes.
	docker ps --filter="name=$(CLUSTER)-*"

.PHONY: cluster-workers
cluster-workers: ## List cluster worker Nodes.
	docker ps --filter="name=$(CLUSTER)-worker-*"

.PHONY: cluster-nodes
cluster-nodes: kind ## Get cluster nodes.
	@$(KIND) get nodes --name $(CLUSTER)

.PHONY: stop-control-plane
stop-control-plane: ## Stop control-plane Node.
	docker stop $(CLUSTER)-control-plane

.PHONY: start-control-plane
start-control-plane: ## Start control-plane Node.
	docker start $(CLUSTER)-control-plane

##@ Registry

.PHONY: registry
registry: ## Configure registry auth.
	@for node in $$(make -s cluster-nodes); do \
		docker cp $(DOCKER_CONFIG) $$node:/var/lib/kubelet/config.json; \
	done

REGISTRY_PULL_SECRET ?= registry
.PHONY: registry-secret
registry-secret: ## Configure registry pull secret.
	@$(KUBECTL) create secret docker-registry $(REGISTRY_PULL_SECRET) --from-file=.dockerconfigjson=$(DOCKER_CONFIG) --dry-run=client -o yaml \
		| $(KUBECTL) apply -f -

##@ Failover

MARIADB_INSTANCE ?= mariadb-galera

stop-mariadb-%: ## Stop mariadb Node
	docker stop $(shell kubectl get pod "$(MARIADB_INSTANCE)-$*" -o jsonpath="{.spec.nodeName}")

start-mariadb-%: ## Stop mariadb Node
	docker start $(shell kubectl get pod "$(MARIADB_INSTANCE)-$*" -o jsonpath="{.spec.nodeName}")

.PHONY: stop-all-mariadb
stop-all-mariadb: ## Stop all mariadb Nodes
	@for ((i=0; i<$(shell kubectl get mariadb "$(MARIADB_INSTANCE)" -o jsonpath='{.spec.replicas}'); i++)); do make -s "stop-mariadb-$$i"; done
	@make -s cluster-workers

.PHONY: start-all-mariadb
start-all-mariadb: ## Stop all mariadb Nodes
	@for ((i=0; i<$(shell kubectl get mariadb "$(MARIADB_INSTANCE)" -o jsonpath='{.spec.replicas}'); i++)); do make -s "start-mariadb-$$i"; done
	@make -s cluster-workers

POD ?= mariadb-repl-0
.PHONY: delete-pod
delete-pod: ## Continiously delete a Pod.
	@while true; do kubectl delete pod $(POD); sleep 1; done;

##@ Helm

CT_IMG ?= quay.io/helmpack/chart-testing:v3.5.0 
.PHONY: helm-lint
helm-lint: ## Lint Helm charts.
	docker run --rm --workdir /repo -v $(shell pwd):/repo $(CT_IMG) ct lint --config .github/config/ct.yml 

.PHONY: helm-chart-version
helm-chart-version: yq ## Get helm chart version.
	@cat $(HELM_DIR)/Chart.yaml | $(YQ) e ".version"

##@ Install

PROMETHEUS_VERSION ?= "58.3.1"

.PHONY: install-prometheus-crds
install-prometheus-crds: cluster-ctx  ## Install Prometheus CRDs.
	kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-$(PROMETHEUS_VERSION)/charts/kube-prometheus-stack/charts/crds/crds/crd-servicemonitors.yaml

.PHONY: install-prometheus
install-prometheus: cluster-ctx ## Install kube-prometheus-stack helm chart.
	@PROMETHEUS_VERSION=$(PROMETHEUS_VERSION) ./hack/install_prometheus.sh

CERT_MANAGER_VERSION ?= "v1.14.5"
.PHONY: install-cert-manager
install-cert-manager: cluster-ctx ## Install cert-manager helm chart.
	@CERT_MANAGER_VERSION=$(CERT_MANAGER_VERSION) ./hack/install_cert_manager.sh

METALLB_VERSION ?= "0.14.5"
.PHONY: install-metallb
install-metallb: cluster-ctx ## Install metallb helm chart.
	@METALLB_VERSION=$(METALLB_VERSION) ./hack/install_metallb.sh

MINIO_VERSION ?= "5.2.0"
.PHONY: install-minio
install-minio: cert-minio ## Install minio helm chart.
	@MINIO_VERSION=$(MINIO_VERSION) ./hack/install_minio.sh

.PHONY: install-crds
install-crds: cluster-ctx manifests kustomize ## Install CRDs.
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true --force-conflicts -f -

.PHONY: uninstall-crds
uninstall-crds: cluster-ctx manifests kustomize ## Uninstall CRDs.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: install-config
install-config: cluster-ctx  ## Install common configuration.
	kubectl apply -f examples/manifests/config

.PHONY: serviceaccount
serviceaccount: cluster-ctx  ## Create long-lived ServiceAccount token for development.
	@./hack/create_serviceaccount.sh

.PHONY: serviceaccount-token
serviceaccount-get: cluster-ctx ## Get ServiceAccount token for development.
	$(KUBECTL) get secret mariadb-operator -o jsonpath="{.data.token}" | base64 -d

.PHONY: storageclass
storageclass: cluster-ctx  ## Create StorageClass that allows volume expansion.
	$(KUBECTL) apply -f ./hack/manifests/storageclass.yaml

.PHONY: install
install: cluster-ctx install-crds install-config install-prometheus-crds serviceaccount storageclass cert docker-dev ## Install everything you need for local development.

.PHONY: install-ent
install-ent: cluster-ctx install-crds install-config install-prometheus-crds serviceaccount storageclass cert docker-dev-ent ## Install everything you need for local enterprise development.

##@ Deploy

.PHONY: deploy-ent
deploy-ent: manifests kustomize cluster-ctx ## Deploy enterprise controller.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_ENT}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply --server-side=true -f -

.PHONY: undeploy-ent
undeploy-ent: cluster-ctx ## Undeploy enterprise controller.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Sysbench

.PHONY: sysbench-prepare-repl
sysbench-prepare-repl: ## Prepare sysbench tests for replication.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/replication/sbtest-repl_database.yaml
	$(KUBECTL) wait --for=condition=ready database sbtest-repl
	$(KUBECTL) apply -f ./hack/manifests/sysbench/replication/sysbench-prepare-repl_job.yaml

.PHONY: sysbench-repl
sysbench-repl: ## Run sysbench tests for replication.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/replication/sysbench-repl_cronjob.yaml
	$(KUBECTL) create job sysbench-repl --from cronjob/sysbench-repl

.PHONY: sysbench-prepare-galera
sysbench-prepare-galera: ## Prepare sysbench tests for Galera.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/galera/sbtest-galera_database.yaml
	$(KUBECTL) wait --for=condition=ready database sbtest-galera
	$(KUBECTL) apply -f ./hack/manifests/sysbench/galera/sysbench-prepare-galera_job.yaml

.PHONY: sysbench-galera
sysbench-galera: ## Run sysbench tests for Galera.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/galera/sysbench-galera_cronjob.yaml
	$(KUBECTL) create job sysbench-galera --from cronjob/sysbench-galera

##@ Examples

GITHUB_USER := mariadb-operator
GITHUB_REPOSITORY := mariadb-operator
GITHUB_BRANCH ?= main

.PHONY: example-flux
example-flux: flux ## Install flux example.
	$(FLUX) bootstrap github \
		--owner=$(GITHUB_USER) \
		--repository=$(GITHUB_REPOSITORY)\
		--branch=$(GITHUB_BRANCH) \
		--path=./examples/flux/clusters/production \
		--personal