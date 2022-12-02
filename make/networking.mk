##@ Networking

MARIADB_IP ?= 127.0.0.1
MARIADB_HOST ?= mariadb.default.svc.cluster.local
.PHONY: mdb-add-host
mdb-add-host: ## Add mariadb host to /etc/hosts.
	@./hack/add_host.sh $(MARIADB_IP) $(MARIADB_HOST)

MARIADB_TEST_HOST ?= mariadb-test.default.svc.cluster.local
.PHONY: mdb-add-test-host
mdb-add-test-host: ## Add mariadb test hosts to /etc/hosts.
	@./hack/add_host.sh $(MARIADB_IP) $(MARIADB_TEST_HOST)

MARIADB_NAMESPACE ?= default
MARIADB_POD ?= mariadb-0
MARIADB_PORT ?= 3306
.PHONY: mdb-port-forward
mdb-port-forward: ## Port forward mariadb pod.
	@kubectl port-forward -n $(MARIADB_NAMESPACE) $(MARIADB_POD) $(MARIADB_PORT)