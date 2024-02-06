##@ Networking

.PHONY: cidr
cidr: ## Get CIDR used by KIND.
	@./hack/display_cidr.sh

.PHONY: host-mariadb
host-mariadb: ## Add mariadb hosts to /etc/hosts.
	@./hack/add_host.sh 10 mariadb-0.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 11 mariadb-1.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 12 mariadb-2.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 13 mariadb-3.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 20 mariadb.default.svc.cluster.local
	@./hack/add_host.sh 30 mariadb-primary.default.svc.cluster.local
	@./hack/add_host.sh 31 mariadb-secondary.default.svc.cluster.local
	@./hack/add_host.sh 40 mariadb.mariadb.svc.cluster.local

.PHONY: host-mdb-test
host-mdb-test: ## Add MariaDB test hosts to /etc/hosts.
	@./hack/add_host.sh 45 mdb-test.default.svc.cluster.local

.PHONY: host-mxs-repl-test
host-mxs-repl-test: ## Add MaxScale replication test hosts to /etc/hosts.
	@./hack/add_host.sh 50 mxs-repl-0.mxs-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 51 mxs-repl-1.mxs-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 52 mxs-repl-2.mxs-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 53 mxs-repl-3.mxs-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 54 mxs-repl.default.svc.cluster.local
	@./hack/add_host.sh 55 mxs-repl-primary.default.svc.cluster.local
	@./hack/add_host.sh 60 mxs-repl-maxscale-0.mxs-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 61 mxs-repl-maxscale-1.mxs-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 62 mxs-repl-maxscale-2.mxs-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 63 mxs-repl-maxscale-3.mxs-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 64 mxs-repl-maxscale.default.svc.cluster.local

.PHONY: host-mxs-galera-test
host-mxs-galera-test: ## Add MaxScale Galera test hosts to /etc/hosts.
	@./hack/add_host.sh 70 mxs-galera-0.mxs-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 71 mxs-galera-1.mxs-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 72 mxs-galera-2.mxs-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 73 mxs-galera-3.mxs-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 74 mxs-galera.default.svc.cluster.local
	@./hack/add_host.sh 75 mxs-galera-primary.default.svc.cluster.local
	@./hack/add_host.sh 80 mxs-galera-maxscale-0.mxs-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 81 mxs-galera-maxscale-1.mxs-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 82 mxs-galera-maxscale-2.mxs-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 83 mxs-galera-maxscale-3.mxs-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 84 mxs-galera-maxscale.default.svc.cluster.local

.PHONY: host-mariadb-repl
host-mariadb-repl: ## Add mariadb repl hosts to /etc/hosts.
	@./hack/add_host.sh 110 mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 111 mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 112 mariadb-repl-2.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 113 mariadb-repl-3.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 120 mariadb-repl.default.svc.cluster.local
	@./hack/add_host.sh 130 mariadb-repl-primary.default.svc.cluster.local
	@./hack/add_host.sh 131 mariadb-repl-secondary.default.svc.cluster.local

.PHONY: host-mariadb-galera
host-mariadb-galera: ## Add mariadb galera hosts to /etc/hosts.
	@./hack/add_host.sh 140 mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 141 mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 142 mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 143 mariadb-galera-3.mariadb-galera-internal.default.svc.cluster.local	
	@./hack/add_host.sh 150 mariadb-galera.default.svc.cluster.local
	@./hack/add_host.sh 160 mariadb-galera-primary.default.svc.cluster.local
	@./hack/add_host.sh 161 mariadb-galera-secondary.default.svc.cluster.local

.PHONY: host-monitoring
host-monitoring: ## Add monitoring hosts to /etc/hosts.
	@./hack/add_host.sh 190 prometheus
	@./hack/add_host.sh 191 grafana

.PHONY: host-minio
host-minio: ## Add minio hosts to /etc/hosts.
	@./hack/add_host.sh 200 minio
	@./hack/add_host.sh 201 minio-console

.PHONY: host-maxscale-repl
host-maxscale-repl: ## Add maxscale-repl hosts to /etc/hosts.
	@./hack/add_host.sh 210 maxscale-repl-0.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 211 maxscale-repl-1.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 212 maxscale-repl-2.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 213 maxscale-repl-3.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 214 maxscale-repl.default.svc.cluster.local
	@./hack/add_host.sh 215 mariadb-repl-maxscale-0.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 216 mariadb-repl-maxscale-1.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 217 mariadb-repl-maxscale-2.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 218 mariadb-repl-maxscale-3.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 219 mariadb-repl-maxscale.default.svc.cluster.local

.PHONY: host-maxscale-galera
host-maxscale-galera: ## Add maxscale-galera hosts to /etc/hosts.
	@./hack/add_host.sh 220 maxscale-galera-0.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 221 maxscale-galera-1.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 222 maxscale-galera-2.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 223 maxscale-galera-3.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 224 maxscale-galera.default.svc.cluster.local
	@./hack/add_host.sh 225 mariadb-galera-maxscale-0.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 226 mariadb-galera-maxscale-1.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 227 mariadb-galera-maxscale-2.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 228 mariadb-galera-maxscale-3.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 229 mariadb-galera-maxscale.default.svc.cluster.local


.PHONY: host
host: host-mariadb host-mariadb-repl host-mariadb-galera host-monitoring host-minio host-maxscale-repl host-maxscale-galera ## Configure hosts for local development.

.PHONY: host-test
host-test: host host-mdb-test host-mxs-repl-test host-mxs-galera-test ## Configure hosts for local tests.

.PHONY: net
net: install-metallb host host-test ## Configure networking for local development.