HELM_DIR ?= deploy/charts/mariadb-operator
CLUSTER ?= mdb

##@ Cluster

KIND_CONFIG ?= hack/config/kind.yaml
KIND_IMAGE ?= kindest/node:v1.28.0

.PHONY: cluster
cluster: kind ## Create a single node kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --image $(KIND_IMAGE)

.PHONY: cluster-ha
cluster-ha: kind ## Create a HA kind cluster.
	$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

.PHONY: cluster-delete
cluster-delete: kind ## Delete the kind cluster.
	$(KIND) delete cluster --name $(CLUSTER)

.PHONY: kubectl cluster-ctx
cluster-ctx: ## Sets cluster context.
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

##@ Registry

.PHONY: registry
registry: ## Configure registry auth.
	@for node in $$(make -s cluster-nodes); do \
		docker cp $(DOCKER_CONFIG) $$node:/var/lib/kubelet/config.json; \
	done

OCP_REGISTRY_URL ?= https://index.docker.io/v1/
.PHONY: openshift-registry
openshift-registry-add: oc jq ## Add catalog registry in OpenShift global config.
	$(OC) extract secret/pull-secret -n openshift-config --confirm
	@cat .dockerconfigjson | $(JQ) -c \
		--argjson registryauth '$(shell cat $(DOCKER_CONFIG) | $(JQ) '.auths["$(OCP_REGISTRY_URL)"]')' '.auths["$(OCP_REGISTRY_URL)"] |= . + $$registryauth' \
		> .new_dockerconfigjson 
	$(OC) set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=.new_dockerconfigjson 
	@rm .dockerconfigjson .new_dockerconfigjson 

.PHONY: openshift-registry
openshift-registry: ## Setup registries in OpenShift global config.
	$(MAKE) openshift-registry-add OCP_REGISTRY_URL=https://index.docker.io/v1/
	$(MAKE) openshift-registry-add OCP_REGISTRY_URL=us-central1-docker.pkg.dev

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

PROMETHEUS_VERSION ?= "55.5.0"

.PHONY: install-prometheus-crds
install-prometheus-crds: cluster-ctx  ## Install Prometheus CRDs.
	kubectl apply -f https://raw.githubusercontent.com/prometheus-community/helm-charts/kube-prometheus-stack-$(PROMETHEUS_VERSION)/charts/kube-prometheus-stack/charts/crds/crds/crd-servicemonitors.yaml

.PHONY: install-prometheus
install-prometheus: cluster-ctx ## Install kube-prometheus-stack helm chart.
	@./hack/install_prometheus.sh

CERT_MANAGER_VERSION ?= "v1.9.1"
.PHONY: install-cert-manager
install-cert-manager: cluster-ctx ## Install cert-manager helm chart.
	@./hack/install_cert_manager.sh

METALLB_VERSION ?= "0.13.9"
.PHONY: install-metallb
install-metallb: cluster-ctx ## Install metallb helm chart.
	@./hack/install_metallb.sh

MINIO_VERSION ?= "5.0.14"
.PHONY: install-minio
install-minio: cluster-ctx cert-minio ## Install minio helm chart.
	@./hack/install_minio.sh

.PHONY: install-crds
install-crds: cluster-ctx manifests kustomize ## Install CRDs.
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side=true --force-conflicts -f -

.PHONY: uninstall-crds
uninstall-crds: cluster-ctx manifests kustomize ## Uninstall CRDs.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: install-samples
install-samples: cluster-ctx  ## Install sample configuration.
	kubectl apply -f examples/manifests/config

.PHONY: serviceaccount
serviceaccount: cluster-ctx  ## Create long-lived ServiceAccount token for development.
	@./hack/create_serviceaccount.sh

.PHONY: install
install: cluster-ctx install-crds install-samples install-prometheus-crds serviceaccount cert docker-dev ## Install everything you need for local development.

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
	flux bootstrap github \
		--owner=$(GITHUB_USER) \
		--repository=$(GITHUB_REPOSITORY)\
		--branch=$(GITHUB_BRANCH) \
		--path=./examples/flux/clusters/production \
		--personal