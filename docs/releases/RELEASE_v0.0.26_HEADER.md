🛠️ In this release, we have considerable shape our CRD APIs to eventually converge to a `v1beta1` version. We have introduced multiple new fields with brand new functionallity, each of them documented in the PRs below with the relevant [API reference](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/API_REFERENCE.md).

🙌 We have also significantly changed the MariaDB Galera architecture to improve its overall stability and robustness. These changes include:
- Liveness and readiness probes are now delegated to the `agent`. This enables better Galera cluster recovery and new features like `availableWhenDonor`.
- `agent` and `init` imags are part now of the `mariadb-operator` glued CLI
- Introduce a Galera init `Job` to perform initialization tasks.

The automated Galera cluster recovery is now way more predictable, robust and configurable. The user may now specify a `minClusterSize`, either absolute number of replicas (`2`) or relative (`50%`), that will tell the operator when the cluster is considered unhealthy so it can perform the recovery process after a given period of time defined by `clusterHealthyTimeout`. Refer to the [Galera documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/GALERA.md) for further detail

Some more new features, now related to the storage: The operator is now able to resize the volumes used by `MariaDB` wihout affecting its available, so you can seamlessly scale the storage size and the volume of data stored by your applications. We have also simplfied our storage API to make it ridiculously simple, take a look at the [storage documentation](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/STORAGE.md). 

To enable better integrations with cloud providers and other CNCF projects, we have introduced support for `inheritMetadata` and `serviceAccountName` fields in the `Backup`, `Restore` and `SqlJob` resources. This allows you to use dedicated metadata (i.e. labels and annotations) and `ServiceAccounts` in all of our batch `Job`/`CronJob` resources.

🛠️ In order to achieve this, we have introduced some **BREAKING CHANGES**, please take a look at the upgrade guide. We've provided a migration script to facilitate the transition:
- **[UPGRADE GUIDE v0.0.26](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_v0.0.26.md)**


🤝 We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

👥 Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.