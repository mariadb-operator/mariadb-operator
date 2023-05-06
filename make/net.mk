##@ Networking

.PHONY: host-mariadb
host-mariadb:  ## Add mariadb hosts to /etc/hosts.
	@./hack/add_host.sh 172.18.0.10 mariadb-0.mariadb.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.11 mariadb-1.mariadb.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.12 mariadb-2.mariadb.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.13 mariadb-3.mariadb.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.20 mariadb.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.30 primary-mariadb.default.svc.cluster.local

.PHONY: host-mariadb-test
host-mariadb-test: ## Add mariadb test hosts to /etc/hosts.
	@./hack/add_host.sh 172.18.0.100 mariadb-test.default.svc.cluster.local

.PHONY: host-mariadb-repl
host-mariadb-repl: ## Add mariadb repl hosts to /etc/hosts.
	@./hack/add_host.sh 172.18.0.110 mariadb-repl-0.mariadb-repl.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.111 mariadb-repl-1.mariadb-repl.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.112 mariadb-repl-2.mariadb-repl.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.120 mariadb-repl.default.svc.cluster.local
	@./hack/add_host.sh 172.18.0.130 primary-mariadb-repl.default.svc.cluster.local

.PHONY: net
net: install-metallb host-mariadb host-mariadb-repl ## Configure networking for local development.

.PHONY: net-test
net-test: install-metallb host-mariadb-test host-mariadb-repl ## Configure networking for tests.