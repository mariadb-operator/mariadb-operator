# Standalone MariaDB

MariaDB Operator allows you to configure standalone MariaDB Server instances. To achieve this, you can either omit the `replicas` field or set it to `1`:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: password

  replicas: 1

  port: 3306

  storage:
    size: 1Gi

  myCnf: |
    [mariadb]
    bind-address=*
    default_storage_engine=InnoDB
    binlog_format=row
    innodb_autoinc_lock_mode=2
    innodb_buffer_pool_size=800M
    max_allowed_packet=256M

  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      memory: 1Gi

  metrics:
    enabled: true
```

Whilst this can be useful for development and testing, it is not recommended for production use because of the following reasons:

* Single point of failure
* Upgrades require downtime
* Only vertical scaling is possible

For achieving high availability, we recommend deploying a Galera cluster. Refer to the [Galera](./galera.md) and [High Availability](./high_availability.md) sections for more information.

