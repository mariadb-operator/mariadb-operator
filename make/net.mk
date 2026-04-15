##@ Networking

.PHONY: cidr
cidr: ## Get CIDR used by KIND.
	@./hack/display_cidr.sh

.PHONY: host-mariadb
host-mariadb: ## Add mariadb hosts to /etc/hosts.
	@./hack/add_host.sh 0 10 mariadb-0.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 11 mariadb-1.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 12 mariadb-2.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 13 mariadb-3.mariadb-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 20 mariadb.default.svc.cluster.local
	@./hack/add_host.sh 0 30 mariadb-primary.default.svc.cluster.local
	@./hack/add_host.sh 0 31 mariadb-secondary.default.svc.cluster.local
	@./hack/add_host.sh 0 40 mariadb.mariadb.svc.cluster.local

.PHONY: host-mdb-test
host-mdb-test: ## Add MariaDB test hosts to /etc/hosts.
	@./hack/add_host.sh 0 45 mdb-test-0.mdb-test-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 46 mdb-test.default.svc.cluster.local

.PHONY: host-mdb-emulated-external-test
host-mdb-emulated-external-test: ## Add MariaDB test hosts to /etc/hosts.
	@./hack/add_host.sh 0 47 mdb-emulate-external-test.default.svc.cluster.local
	@./hack/add_host.sh 0 48 mdb-emulate-external-test-0.mdb-emulate-external-test-internal.default.svc.cluster.local

.PHONY: host-mxs-test
host-mxs-test: ## Add MaxScale test hosts to /etc/hosts.
	@./hack/add_host.sh 0 50 mxs-test-0.mxs-test-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 51 mxs-test.default.svc.cluster.local

.PHONY: host-azurite
host-azurite: ## Add Azurite test hosts to /etc/hosts.
	@./hack/add_host.sh 0 60 azurite.default.svc.cluster.local

.PHONY: host-mariadb-repl
host-mariadb-repl: ## Add mariadb repl hosts to /etc/hosts.
	@./hack/add_host.sh 0 110 mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 111 mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 112 mariadb-repl-2.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 113 mariadb-repl-3.mariadb-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 120 mariadb-repl.default.svc.cluster.local
	@./hack/add_host.sh 0 130 mariadb-repl-primary.default.svc.cluster.local
	@./hack/add_host.sh 0 131 mariadb-repl-secondary.default.svc.cluster.local

.PHONY: host-mariadb-galera
host-mariadb-galera: ## Add mariadb galera hosts to /etc/hosts.
	@./hack/add_host.sh 0 140 mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 141 mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 142 mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 143 mariadb-galera-3.mariadb-galera-internal.default.svc.cluster.local	
	@./hack/add_host.sh 0 150 mariadb-galera.default.svc.cluster.local
	@./hack/add_host.sh 0 160 mariadb-galera-primary.default.svc.cluster.local
	@./hack/add_host.sh 0 161 mariadb-galera-secondary.default.svc.cluster.local

.PHONY: host-mariadb-galera-test
host-mariadb-galera-test: ## Add mariadb galera test hosts to /etc/hosts.
	@./hack/add_host.sh 0 165 mariadb-galera-test-0.mariadb-galera-test-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 166 mariadb-galera-test-1.mariadb-galera-test-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 167 mariadb-galera-test-2.mariadb-galera-test-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 168 mariadb-galera-test.default.svc.cluster.local
	@./hack/add_host.sh 0 169 mariadb-galera-test-primary.default.svc.cluster.local
	@./hack/add_host.sh 0 170 mariadb-galera-test-secondary.default.svc.cluster.local

.PHONY: host-monitoring
host-monitoring: ## Add monitoring hosts to /etc/hosts.
	@./hack/add_host.sh 0 190 prometheus
	@./hack/add_host.sh 0 191 grafana

.PHONY: host-minio
host-minio: ## Add minio hosts to /etc/hosts.
	@./hack/add_host.sh 0 200 minio
	@./hack/add_host.sh 0 201 minio-console

.PHONY: host-maxscale-repl
host-maxscale-repl: ## Add maxscale-repl hosts to /etc/hosts.
	@./hack/add_host.sh 0 210 maxscale-repl-0.maxscale-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 211 maxscale-repl-1.maxscale-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 212 maxscale-repl-2.maxscale-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 213 maxscale-repl-3.maxscale-repl-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 214 maxscale-repl.default.svc.cluster.local
	@./hack/add_host.sh 0 215 mariadb-repl-maxscale-0.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 216 mariadb-repl-maxscale-1.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 217 mariadb-repl-maxscale-2.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 218 mariadb-repl-maxscale-3.mariadb-repl-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 219 mariadb-repl-maxscale.default.svc.cluster.local

.PHONY: host-maxscale-galera
host-maxscale-galera: ## Add maxscale-galera hosts to /etc/hosts.
	@./hack/add_host.sh 0 220 maxscale-galera-0.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 221 maxscale-galera-1.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 222 maxscale-galera-2.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 223 maxscale-galera-3.maxscale-galera-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 224 maxscale-galera.default.svc.cluster.local
	@./hack/add_host.sh 0 225 mariadb-galera-maxscale-0.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 226 mariadb-galera-maxscale-1.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 227 mariadb-galera-maxscale-2.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 228 mariadb-galera-maxscale-3.mariadb-galera-maxscale-internal.default.svc.cluster.local
	@./hack/add_host.sh 0 229 mariadb-galera-maxscale.default.svc.cluster.local

.PHONY: host-maxscale-gui
host-maxscale-gui: ## Add maxscale GUI hosts to /etc/hosts.
	@./hack/add_host.sh 0 230 maxscale-repl-gui.default.svc.cluster.local
	@./hack/add_host.sh 0 231 maxscale-galera-gui.default.svc.cluster.local

.PHONY: host-multi-cluster
host-multi-cluster: ## Add multi-cluster hosts to /etc/hosts.
	@./hack/add_host.sh 1 10 mariadb-eu-south-primary.default.svc.cluster.local
	@./hack/add_host.sh 1 11 mariadb-eu-south-0.mariadb-eu-south-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 12 mariadb-eu-south-1.mariadb-eu-south-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 13 mariadb-eu-south-2.mariadb-eu-south-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 14 mariadb-eu-south-3.mariadb-eu-south-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 15 mariadb-eu-central-primary.default.svc.cluster.local
	@./hack/add_host.sh 1 16 mariadb-eu-central-0.mariadb-eu-central-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 17 mariadb-eu-central-1.mariadb-eu-central-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 18 mariadb-eu-central-2.mariadb-eu-central-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 19 mariadb-eu-central-3.mariadb-eu-central-internal.default.svc.cluster.local

.PHONY: host-multi-cluster-mxs
host-multi-cluster-mxs: ## Add multi-cluster maxscale hosts to /etc/hosts.
	@./hack/add_host.sh 1 20 maxscale-eu-south.default.svc.cluster.local
	@./hack/add_host.sh 1 21 maxscale-eu-south-gui.default.svc.cluster.local
	@./hack/add_host.sh 1 22 maxscale-eu-south-0.maxscale-eu-south-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 23 maxscale-eu-south-1.maxscale-eu-south-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 24 maxscale-eu-central.default.svc.cluster.local
	@./hack/add_host.sh 1 25 maxscale-eu-central-gui.default.svc.cluster.local
	@./hack/add_host.sh 1 26 maxscale-eu-central-0.maxscale-eu-central-internal.default.svc.cluster.local
	@./hack/add_host.sh 1 27 maxscale-eu-central-1.maxscale-eu-central-internal.default.svc.cluster.local

.PHONY: host
host: host-mariadb host-mdb-test host-mdb-emulated-external-test host-mxs-test host-mariadb-repl host-mariadb-galera host-mariadb-galera-test host-monitoring host-minio host-maxscale-repl host-maxscale-galera host-maxscale-gui host-multi-cluster host-multi-cluster-mxs host-azurite ## Configure hosts for local development.

.PHONY: net
net: install-metallb host ## Configure networking for local development.
