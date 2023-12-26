# High availability

This operator supports the following High Availability modes:
- **Single master HA via [SemiSync Replication](../examples/manifests/mariadb_v1alpha1_mariadb_replication.yaml)**: The primary node allows both reads and writes, while secondary nodes only allow reads.
- **Multi master HA via [Galera](./GALERA.md)**: All nodes support reads and writes, but it is recommended to perform writes in a single primary for preventing deadlocks.

In order to address nodes, `mariadb-operator` provides you with the following Kubernetes `Services`:
- `<mariadb-name>`: To be used for read requests. It will point to all nodes. 
- `<mariadb-name>-primary`: To be used for write requests. It will point to a single node, the primary.
- `<mariadb-name>-secondary`: To be used for read requests. It will point to all nodes, except the primary.

Whenever the primary changes, either by the user or by the operator, both the `<mariadb-name>-primary` and `<mariadb-name>-secondary` `Services` will be automatically updated by the operator to address the right nodes.

The primary may be manually changed by the user at any point by updating the `spec.[replication|galera].primary.podIndex` field. Alternatively,  automatic primary failover can be enabled by setting `spec.[replication|galera].primary.automaticFailover`, which will make the operator to switch primary whenever the primary `Pod` goes down.