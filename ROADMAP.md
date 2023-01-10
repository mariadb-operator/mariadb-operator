# 🛣️ Roadmap

- Improve and fully automate the CI/CD to ensure the quality of the artifacts. Become a serious open source project in this matter.
- Leveraging the previous automation, support deploying the operator using the [openshift OLM](https://docs.openshift.com/container-platform/4.8/operators/understanding/olm/olm-understanding-olm.html).
  - https://github.com/mmontes11/mariadb-operator/issues/10
- Abstract both [mysqldump](https://dev.mysql.com/doc/refman/8.0/en/mysqldump.html) and [mariabackup](https://mariadb.com/kb/en/mariabackup-overview/) in an easy to use CLI: `mariactl`. The binary will be released along with the operator in the new CI/CD:
  - https://github.com/mmontes11/mariadb-operator/issues/16
- Making use of `mariactl`, abstract the most relevant features of `mariabackup` via Kubernetes CRDs, such as incremental backups:
  - https://github.com/mmontes11/mariadb-operator/issues/17
- The operator has recently been refactored to easily support new storage types for the backups. The next one to be supported will be S3:
  - https://github.com/mmontes11/mariadb-operator/issues/6
- Create a documentation site hosted in GitHub Pages, something like [this](https://gateway-api.sigs.k8s.io/). It would be generated from markdown by the new CI/CD:
   - https://github.com/mmontes11/mariadb-operator/issues/21
 - High availability support via [MariaDB Galera](https://mariadb.com/kb/en/what-is-mariadb-galera-cluster/):
    - https://github.com/mmontes11/mariadb-operator/issues/4
