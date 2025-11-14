CLUSTER ?= mdb

##@ Cluster

KIND_CONFIG ?= hack/config/kind.yaml
KIND_IMAGE ?= kindest/node:v1.34.0

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
	$(DOCKER) ps --filter="name=$(CLUSTER)-*"

.PHONY: cluster-workers
cluster-workers: ## List cluster worker Nodes.
	$(DOCKER) ps --filter="name=$(CLUSTER)-worker-*"

.PHONY: cluster-nodes
cluster-nodes: kind ## Get cluster nodes.
	@$(KIND) get nodes --name $(CLUSTER)

.PHONY: stop-control-plane
stop-control-plane: ## Stop control-plane Node.
	$(DOCKER) stop $(CLUSTER)-control-plane

.PHONY: start-control-plane
start-control-plane: ## Start control-plane Node.
	$(DOCKER) start $(CLUSTER)-control-plane

##@ Registry

.PHONY: registry
registry: ## Configure registry auth.
	@for node in $$(make -s cluster-nodes); do \
		$(DOCKER) cp $(DOCKER_CONFIG) $$node:/var/lib/kubelet/config.json; \
	done

REGISTRY_PULL_SECRET ?= registry
.PHONY: registry-secret
registry-secret: ## Configure registry pull secret.
	@$(KUBECTL) create secret docker-registry $(REGISTRY_PULL_SECRET) --from-file=.dockerconfigjson=$(DOCKER_CONFIG) --dry-run=client -o yaml \
		| $(KUBECTL) apply -f -

##@ Failover

MARIADB_INSTANCE ?= mariadb-repl

stop-mariadb-%: ## Stop mariadb Node
	$(DOCKER) stop $(shell kubectl get pod "$(MARIADB_INSTANCE)-$*" -o jsonpath="{.spec.nodeName}")

start-mariadb-%: ## Stop mariadb Node
	$(DOCKER) start $(shell kubectl get pod "$(MARIADB_INSTANCE)-$*" -o jsonpath="{.spec.nodeName}")

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
	@while true; do kubectl delete pod --force $(POD); sleep .5; done;

##@ Install

PROMETHEUS_VERSION ?= "70.0.2"

.PHONY: install-prometheus-crds
install-prometheus-crds: cluster-ctx  ## Install Prometheus CRDs.
	kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-$(PROMETHEUS_VERSION)/charts/kube-prometheus-stack/charts/crds/crds/crd-servicemonitors.yaml

.PHONY: install-prometheus
install-prometheus: helm cluster-ctx ## Install kube-prometheus-stack helm chart.
	@PROMETHEUS_VERSION=$(PROMETHEUS_VERSION) HELM=$(HELM) ./hack/install_prometheus.sh

CERT_MANAGER_VERSION ?= "v1.17.1"
.PHONY: install-cert-manager
install-cert-manager: helm cluster-ctx ## Install cert-manager helm chart.
	@CERT_MANAGER_VERSION=$(CERT_MANAGER_VERSION) HELM=$(HELM) ./hack/install_cert_manager.sh

TRUST_MANAGER_VERSION ?= "v0.16.0"
.PHONY: install-trust-manager
install-trust-manager: helm cluster-ctx install-cert-manager ## Install trust-manager helm chart.
	@TRUST_MANAGER_VERSION=$(TRUST_MANAGER_VERSION) HELM=$(HELM) ./hack/install_trust_manager.sh

METALLB_VERSION ?= "0.14.9"
.PHONY: install-metallb
install-metallb: helm cluster-ctx ## Install metallb helm chart.
	@METALLB_VERSION=$(METALLB_VERSION) HELM=$(HELM) ./hack/install_metallb.sh

MINIO_VERSION ?= "5.4.0"
.PHONY: install-minio
install-minio: helm cert-minio ## Install minio helm chart.
	@MINIO_VERSION=$(MINIO_VERSION) HELM=$(HELM) ./hack/install_minio.sh

.PHONY: install-snapshotter
install-snapshotter: ## Install external-snapshotter.
	$(KUBECTL) create namespace external-snapshotter --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build hack/manifests/external-snapshotter | $(KUBECTL)  apply --server-side=true --force-conflicts -f -

.PHONY: install-csi-hostpath
install-csi-hostpath: install-snapshotter ## Install csi-hostpath.
	@./hack/install_csi_driver_host_path.sh

.PHONY: install-crds
install-crds: cluster-ctx manifests kustomize ## Install CRDs.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply --server-side=true --force-conflicts -f -

.PHONY: uninstall-crds
uninstall-crds: cluster-ctx manifests kustomize ## Uninstall CRDs.
	$(KUSTOMIZE) build config/crd | $(KUBECTL)  delete --ignore-not-found=$(ignore-not-found) -f -

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
install: cluster-ctx install-crds install-config install-prometheus-crds serviceaccount storageclass docker-dev ## Install everything you need for local development.

##@ Deploy

.PHONY: deploy
deploy: manifests kustomize cluster-ctx ## Deploy controller using Kustomize.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply --server-side=true -f -

.PHONY: undeploy
undeploy: cluster-ctx ## Undeploy controller using Kustomize.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Sysbench - Standalone

.PHONY: sysbench-sql
sysbench-sql: ## Prepare sysbench SQL resources for standalone.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/standalone/sql
	$(KUBECTL) wait --for=condition=ready database sbtest
	$(KUBECTL) wait --for=condition=ready user sbtest
	$(KUBECTL) wait --for=condition=ready grant sbtest

.PHONY: sysbench-prepare
sysbench-prepare: sysbench-sql ## Prepare sysbench tests for standalone.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/standalone/sysbench-prepare_job.yaml

.PHONY: sysbench
sysbench: ## Run sysbench tests for standalone.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/standalone/sysbench_cronjob.yaml
	$(KUBECTL) create job sysbench --from cronjob/sysbench

##@ Sysbench - Replication

.PHONY: sysbench-sql-repl
sysbench-sql-repl: ## Prepare sysbench SQL resources for replication.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/replication/sql
	$(KUBECTL) wait --for=condition=ready database sbtest-repl
	$(KUBECTL) wait --for=condition=ready user sbtest-repl
	$(KUBECTL) wait --for=condition=ready grant sbtest-repl

.PHONY: sysbench-prepare-repl
sysbench-prepare-repl: sysbench-sql-repl ## Prepare sysbench tests for replication.
	$(KUBECTL) wait --for=condition=ready database sbtest-repl
	$(KUBECTL) apply -f ./hack/manifests/sysbench/replication/sysbench-prepare-repl_job.yaml

.PHONY: sysbench-repl
sysbench-repl: ## Run sysbench tests for replication.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/replication/sysbench-repl_cronjob.yaml
	$(KUBECTL) create job sysbench-repl --from cronjob/sysbench-repl

##@ Sysbench - Galera

.PHONY: sysbench-sql-galera
sysbench-sql-galera: ## Prepare sysbench SQL resources for Galera.
	$(KUBECTL) apply -f ./hack/manifests/sysbench/galera/sql
	$(KUBECTL) wait --for=condition=ready database sbtest-galera
	$(KUBECTL) wait --for=condition=ready user sbtest-galera
	$(KUBECTL) wait --for=condition=ready grant sbtest-galera

.PHONY: sysbench-prepare-galera
sysbench-prepare-galera: sysbench-sql-galera ## Prepare sysbench tests for Galera.
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
