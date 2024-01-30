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

.PHONY: host-mariadb-test
host-mariadb-test: ## Add mariadb test hosts to /etc/hosts.
	@./hack/add_host.sh 100 mariadb-test.default.svc.cluster.local

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

.PHONY: host-maxscale
host-maxscale: ## Add maxscale hosts to /etc/hosts.
	@./hack/add_host.sh 210 maxscale-0.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 211 maxscale-1.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 212 maxscale-2.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 213 maxscale-3.maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 214 maxscale.default.svc.cluster.local
	@./hack/add_host.sh 215 mariadb-repl-maxscale-0.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 216 mariadb-repl-maxscale.default.svc.cluster.local

.PHONY: host-maxscale-galera
host-maxscale-galera: ## Add maxscale-galera hosts to /etc/hosts.
	@./hack/add_host.sh 220 maxscale-galera-0.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 221 maxscale-galera-1.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 222 maxscale-galera-2.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 223 maxscale-galera-3.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 224 maxscale-galera.default.svc.cluster.local
	@./hack/add_host.sh 225 mariadb-galera-maxscale-0.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 226 mariadb-galera-maxscale.default.svc.cluster.local

.PHONY: net
net: install-metallb host-mariadb host-mariadb-test host-mariadb-repl host-mariadb-galera host-monitoring host-minio host-maxscale host-maxscale-galera ## Configure networking for local development.

