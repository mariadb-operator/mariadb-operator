# 🛣️ Roadmap

- ~~Improve and fully automate the CI/CD to ensure the quality of the artifacts. Become a serious open source project in this matter.~~
- ~~Leveraging the previous automation, support deploying the operator using the [openshift OLM](https://docs.openshift.com/container-platform/4.8/operators/understanding/olm/olm-understanding-olm.html).: https://github.com/mariadb-operator/mariadb-operator/issues/10~~
- ~~COMMUNITY REQUEST - Support for custom `my.cnf`: https://github.com/mariadb-operator/mariadb-operator/issues/51~~
- ~~COMMUNITY REQUEST - Support for db initialization scripts: https://github.com/mariadb-operator/mariadb-operator/issues/47~~
- ~~High availability support via [replication](https://mariadb.org/mariadb-k8s-how-to-replicate-mariadb-in-k8s/): https://github.com/mariadb-operator/mariadb-operator/issues/61~~
- ~~High availability support via [Galera](https://mariadb.com/kb/en/what-is-mariadb-galera-cluster/): https://github.com/mariadb-operator/mariadb-operator/issues/4~~
- TLS support. Allow the user to provide certificates via Secrets or automatically issue them with `cert-manager`. Certificate rotation: https://github.com/mariadb-operator/mariadb-operator/issues/137
- The operator has recently been refactored to easily support new storage types for the backups. The next one to be supported will be S3: https://github.com/mariadb-operator/mariadb-operator/issues/6
- Create a documentation site hosted in GitHub Pages, something like [this](https://gateway-api.sigs.k8s.io/). It would be generated from markdown by the new CI/CD: https://github.com/mariadb-operator/mariadb-operator/issues/21