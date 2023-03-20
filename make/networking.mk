##@ Networking

MARIADB_IP ?= 127.0.0.1
.PHONY: mdb-add-hosts
mdb-add-hosts: ## Add mariadb hosts to /etc/hosts.
	@./hack/add_host.sh $(MARIADB_IP) mariadb-0.mariadb.default.svc.cluster.local
	@./hack/add_host.sh $(MARIADB_IP) mariadb.default.svc.cluster.local

.PHONY: mdb-add-test-hosts
mdb-add-test-hosts: ## Add mariadb test hosts to /etc/hosts.
	@./hack/add_host.sh $(MARIADB_IP) mariadb-test-0.mariadb-test.default.svc.cluster.local
	@./hack/add_host.sh $(MARIADB_IP) mariadb-test.default.svc.cluster.local
	@./hack/add_host.sh $(MARIADB_IP) mariadb-test-repl-0.mariadb-test.default.svc.cluster.local
	@./hack/add_host.sh $(MARIADB_IP) mariadb-test-repl.default.svc.cluster.local

MARIADB_NAMESPACE ?= default
MARIADB_POD ?= mariadb-0
MARIADB_PORT ?= 3306
.PHONY: mdb-port-forward
mdb-port-forward: ## Port forward mariadb pod.
	@kubectl port-forward -n $(MARIADB_NAMESPACE) $(MARIADB_POD) $(MARIADB_PORT)