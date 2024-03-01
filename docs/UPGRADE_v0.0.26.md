# Upgrade guide v0.0.24

> [!NOTE]  
> APIs are currently in `v1alpha1`, which implies that non backward compatible changes might happen. See [Kubernetes API versioning](https://kubernetes.io/docs/reference/using-api/#api-versioning) for more detail.

BREAKING:
https://github.com/mariadb-operator/mariadb-operator/pull/418
https://github.com/mariadb-operator/mariadb-operator/pull/416
https://github.com/mariadb-operator/mariadb-operator/pull/407
https://github.com/mariadb-operator/mariadb-operator/pull/384

Recommended:
- Lower galera recovery timeouts

API:
- Converging to v1beta1
- Multiple fields introduced: relevant API reference linked on each PR below.

MariaDB:
- Significant architecture changes and improved overall stability.
  - Galera init job
  - Liveness and rediness probes delegated to agent. Opens a lot of possibilities like availableWhenDonor
  - mariadb-operator glued CLI: agent and init subcommands
- This changes result in:
  - Improved galera stability
  - Enhanced galera cluster recovery.
    - More robust and predictable. 
    - Define relative minClusterSize to trigger recovery

- Reuse storage volume for config
- Support for MariaDB enterprise image

Backup/Restore/SqlJob:
- ServiceAccount in Jobs
- Support for ContainerTemplate and PodTemplate in Jobs
- Support for inheritMetadata in Jobs

Storage.
- Flexible storage configuration
- Storage resize

Metrics:
- Support for inheritMetadata in Deployment

__BREAKING CHANGE__:  API group has been renamed to `k8s.mariadb.com`

See [API reference](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/API_REFERENCE.md#galerarecovery).