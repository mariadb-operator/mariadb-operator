# Helm

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.32

Helm is the preferred way to install `mariadb-operator` in vanilla Kubernetes clusters (i.e. not OpenShift). This doc aims to provide guidance on how to manage the installation and upgrades of both the CRDs and the operator via Helm charts.

## Table of contents
<!-- toc -->
- [Charts](#charts)
- [Installing CRDs](#installing-crds)
- [Installing the operator](#installing-the-operator)
- [Control-plane](#control-plane)
- [Deployment modes](#deployment-modes)
- [Updates](#updates)
<!-- /toc -->

## Charts

The installation of `mariadb-operator` is splitted in 2 different helm charts for better convenience:
- [`mariadb-operator-crds`](../deploy/charts/mariadb-operator-crds/): Bundles the [`CustomResourceDefinitions`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) required by the operator.
- [`mariadb-operator`](../deploy/charts/mariadb-operator/): Contains all the template manifests required to install the operator.

## Installing CRDs

Helm has certain [limitations when it comes to manage CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations). To address this, we are providing the CRDs in a separate chart, [as recommended by the official Helm documentation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#method-2-separate-charts). This allows us to manage the installation and updates of the CRDs independently from the operator. For example, you can uninstall the operator without impacting your existing `MariaDB` CRDs.

CRDs can be installed in your cluster by running the following commands

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator-crds mariadb-operator/mariadb-operator-crds
```

## Installing the operator

Once the CRDs are available in the cluster, you can proceed to install the operator:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

Refer to the helm chart README for detailed information about all the supported [helm values](./../deploy/charts/mariadb-operator/README.md).

## Control-plane

The `mariadb-operator` is an extension of the Kubernetes control-plane, consisting of the following components that are deployed via Helm:
 
- `operator`: The `mariadb-operator` itself that performs the CRD reconciliation.
- `webhook`: The Kubernetes control-plane delegates CRD validations to this HTTP server. Kubernetes requires TLS to communicate with the webhook server.
- `cert-controller`: Provisions TLS certificates for the webhook. You can see it as a minimal [cert-manager](https://cert-manager.io/) that is intended to work only with the webhook. It is optional and can be replaced by cert-manager.

## Deployment modes

Deployments are highly configurable via the [helm values](./../deploy/charts/mariadb-operator/README.md). In particular the following deployment modes are supported:

#### Cluster-wide

The operator watches CRDs in all namespaces and requires cluster-wide RBAC permissions to operate. This is the default deployment mode, enabled through the default configuration values:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

#### Single namespace

By setting `currentNamespaceOnly=true`, the operator will only watch CRDs in the namespace where it is deployed, and the RBAC permissions will also be limited to that namespace:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator \
  -n databases --create-namespace \
  --set currentNamespaceOnly=true \
  mariadb-operator/mariadb-operator
```

## Updates

> [!IMPORTANT]  
> Make sure you read and understand the [updates documentation](./UPDATES.md) before proceeding to update the operator.

The first step to perform an operator update is upgrading the CRDs:

```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds \
  --version <new-version> \
  mariadb-operator/mariadb-operator-crds
```

Once updated, you may proceed to upgrade the operator:

```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator \
  --version <new-version> \
  mariadb-operator/mariadb-operator 
```

Whenever a new version of the `mariadb-operator` is released, an upgrade guide is linked in the release notes if additional upgrade steps are required. Be sure to review the [release notes](https://github.com/mariadb-operator/mariadb-operator/releases) and follow the version-specific upgrade guides accordingly.