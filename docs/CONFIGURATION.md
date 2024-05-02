# Configuration

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.28

This documentation aims to provide guidance on various configuration aspects across many `mariadb-operator` CRs. 

## Table of contents
<!-- toc -->
- [Passwords](#passwords)
- [Probes](#probes)
<!-- /toc -->

## Passwords

Some CRs require passwords provided as `Secret` references to function properly. For instance, the root password for a `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
``` 

By default, fields like `rootPasswordSecretKeyRef` are optional and defaulted by the operator, resulting in random password generation if not provided:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
    generate: true
``` 

You may choose to explicitly provide a `Secret` reference via `rootPasswordSecretKeyRef` and opt-out fron random password generation by either not providing the `generate` field or setting it to `false`: 

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
    generate: false
``` 

This way, we are telling the operator that we are expecting a `Secret` to be available eventually, enabling the use of GitOps tools to seed the password:
- [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets): The `Secret` is reconciled from a `SealedSecret`, which is decrypted by the sealed-secrets controller.
- [external-secrets](https://github.com/external-secrets/external-secrets): The `Secret` is reconciled fom an `ExternalSecret`, which is read by the external-secrets controller from an external secrets source (Vault, AWS Secrets Manager ...).

## Probes

Kubernetes probes serve as an inversion of control mechanism, enabling the application to communicate its health status to Kubernetes. This enables Kubernetes to take appropriate actions when the application is unhealthy, such as restarting or stop sending traffic to `Pods`.

> [!IMPORTANT]  
> Make sure you check the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) if you are unfamiliar with Kubernetes probes.

Fine tunning of probes for databases running in Kubernetes is critical, you may do so by tweaking the following fields:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  livenessProbe:
    initialDelaySeconds: 20
    periodSeconds: 5
    timeoutSeconds: 5

  readinessProbe:
    initialDelaySeconds: 20
    periodSeconds: 5
    timeoutSeconds: 5
```

There isn't an universally correct default value for these thresholds, so we recommend determining your own based on factors like the compute resources, network, storage, and other aspects of the environment where your `MariaDB` and `MaxScale` instances are running.