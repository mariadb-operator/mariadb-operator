# OpenShift

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.31

This documentation provides guidance on installing the certified [MariaDB Enterprise](https://mariadb.com/products/enterprise/) operator for OpenShift. Please refer to the Red Hat documentation for further detail about the [Red Hat OpenShift Certification program](https://connect.redhat.com/en/partner-with-us/red-hat-openshift-certification).

Operators are deployed into OpenShift by using [Operator Lifecycle Manager](https://docs.redhat.com/en/documentation/openshift_container_platform/4.2/html/operators/understanding-the-operator-lifecycle-manager-olm#olm-overview_olm-understanding-olm), which facilitates the installation, updates, and overall management of their lifecycle.

## Table of contents
<!-- toc -->
- [`PackageManifest`](#packagemanifest)
- [Certified images and registry](#certified-images-and-registry)
- [Channels](#channels)
- [`SecurityContextConstraints`](#channels)
- [Installation in all namespaces](#installation-in-all-namespaces)
- [Installation in specific namespaces](#installation-in-specific-namespaces)
- [Installation via OpenShift console](#installation-via-openshift-console)
<!-- /toc -->

## `PackageManifest`

You can install the certified operator in OpenShift clusters that have the `mariadb-operator-enterprise` `packagemanifest` available. In order to check this, run the following command:

```bash
oc get packagemanifests -n openshift-marketplace mariadb-operator-enterprise

NAME                          CATALOG                 AGE
mariadb-operator-enterprise   Certified Operators     21h
``` 

## Certified images and registry

The certified operator and its related operands make use of certified [Red Hat UBI](https://catalog.redhat.com/software/base-images) based images fully complaint with the [Red Hat OpenShift Certification program](https://connect.redhat.com/en/partner-with-us/red-hat-openshift-certification). Refer to the [image documentation](./DOCKER.md) for getting a list of compatible images.

It is important to note that MariaDB Enterprise server is behind a private registry and therefore accessible only by MariaDB customers. Credentials can be configured by:
- Extract your [global pull secret](https://docs.openshift.com/container-platform/4.10/openshift_images/managing_images/using-image-pull-secrets.html#images-update-global-pull-secret_using-image-pull-secrets):
```bash
oc extract secret/pull-secret -n openshift-config --confirm
``` 
- Obtain a [customer download token](https://mariadb.com/docs/server/deploy/deployment-methods/docker/enterprise-server/).
- Login in the MariaDB registry providing the previous token as password:
```bash
oc registry login \
  --registry="docker-registry.mariadb.com" \
  --auth-basic="<email>:<customer-download-token>" \
  --to=.dockerconfigjson
```
- Update the [global pull secret](https://docs.openshift.com/container-platform/4.10/openshift_images/managing_images/using-image-pull-secrets.html#images-update-global-pull-secret_using-image-pull-secrets):
```bash
oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson
```

Alternatively, instead of updating the [global pull secret](https://docs.openshift.com/container-platform/4.10/openshift_images/managing_images/using-image-pull-secrets.html#images-update-global-pull-secret_using-image-pull-secrets), you can configure credentials in a per-`MariaDB` basis:
- Create a `Secret` with the registry credentials as described in the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
- Provide an `imagePullSecrets` field to your `MariaDB` resource as described in the [registry documentation](./REGISTRY.md).

## Channels

https://olm.operatorframework.io/docs/best-practices/channel-naming/

## `SecurityContextConstraints`

https://github.com/acornett21/kubetruth-operator/blob/98828c17a9866d92b187e2e9d6f804460d75807d/readme.md?plain=1#L24

## Installation in all namespaces

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: mariadb-operator-enterprise
  namespace: openshift-operators
spec:
  channel: fast
  installPlanApproval: Automatic
  name: mariadb-operator-enterprise
  source: certified-operators
  sourceNamespace: openshift-marketplace
  startingCSV: mariadb-operator-enterprise.v0.0.31
``` 

## Installation in specific namespaces

```yaml
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: mariadb-operator-enterprise
  namespace: mariadb
spec:
  targetNamespaces:
  - mariadb
  - mariadb-foo
  - mariadb-bar
  upgradeStrategy: Default
``` 

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: mariadb-operator-enterprise
  namespace: openshift-operators
spec:
  channel: fast
  installPlanApproval: Automatic
  name: mariadb-operator-enterprise
  source: certified-operators
  sourceNamespace: openshift-marketplace
  startingCSV: mariadb-operator-enterprise.v0.0.31
``` 

## Installation via OpenShift console

![Certified Operator](https://mariadb-operator.github.io/mariadb-operator/assets/certified-operator.png)
