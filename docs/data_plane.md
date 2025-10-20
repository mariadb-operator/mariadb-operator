# Data-plane

In order to effectively manage the full lifecycle of both [replication](./replication.md) and [Galera](./galera.md) topologies, the operator relies on a set of components that run alonside the MariaDB instances and expose APIs for remote management. These components are collectively referred to as the "data-plane".

## Table of contents
<!-- toc -->
- [Components](#components)
- [Agent auth methods](#agent-auth-methods)
- [Updates](#updates)
<!-- /toc -->

## Components

The mariadb-operator data-plane components are implemented as lightweight containers that run alongside the MariaDB instances within the same `Pod`. These components are available in the operator image. More preciselly, they are subcommands of the CLI shipped as binary inside the image.

#### Init container

The init container is reponsible for dynamically generating the `Pod`-specifc configuration files before the MariaDB container starts. It also plays a crucial role in the MariaDB container startup, enabling replica recovery for the replication topolology and guaranteeing ordered deployment of `Pods` for the Galera topology.

#### Agent sidecar

The agent sidecar provides an HTTP API that enables the operator to remotely manage MariaDB instances. Through this API, the operator is able to remotely operate the data directory and handle the instance lifecycle, including operations such as replica recovery for the replication topology and cluster recovery for the Galera topology. Additionally, the agent sidecar resolves [liveness and readiness probes](./configuration.md#probes) for the MariaDB container, taking into account the internal state of MariaDB.

It supports [multiple authentication](#agent-auth-methods) methods to ensure that only the operator is able to call the agent API.

## Agent auth methods

As previously mentioned, the agent exposes an API to remotely manage the replication and Galera clusters. The following authentication methods are supported to ensure that only the operator is able to call the agent:

#### `ServiceAccount` based authentication

The operator uses its `ServiceAccount` token as a mean of  authentication for communicating with the agent, which subsequently verifies the token by creating a [`TokenReview` object](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-review-v1/). This is the default authentication method and will be automatically applied by setting:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replication:
    agent:
      kubernetesAuth:
        enabled: true
```
This Kubernetes-native authentication mechanism eliminates the need for the operator to manage credentials, as it relies entirely on Kubernetes for this purpose. However, the drawback is that the agent requires cluster-wide permissions to impersonate the [`system:auth-delegator`](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#other-component-roles) `ClusterRole` and to create [`TokenReviews`](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-review-v1/), which are cluster-scoped objects.

#### Basic authentication

As an alternative, the agent also supports basic authentication:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-repl
spec:
  replication:
    agent:
      basicAuth:
        enabled: true
```

Unlike the [`ServiceAccount` based authentication](#serviceaccount-based-authentication), the operator needs to explicitly generate credentials to authenticate. The advantage of this approach is that it is entirely decoupled from Kubernetes and it does not require cluster-wide permissions on the Kubernetes API.


## Updates

Please refer to the updates documentation for more information about [how to update the data-plane](./updates.md#data-plane-updates).