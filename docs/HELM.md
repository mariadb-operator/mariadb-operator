# Helm

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.32

Helm is the preferred way to install `mariadb-operator` in vanilla Kubernetes clusters (i.e. not OpenShift). This doc aims to provide guidance on how to manage the installation and upgrades of both the CRDs and the operator via Helm charts.

## Table of contents
<!-- toc -->
- [Charts](#charts)
- [Control-plane](#control-plane)
- [Installing CRDs](#installing-crds)
- [Installing the operator](#installing-the-operator)
- [Deployment modes](#deployment-modes)
- [Updates](#updates)
- [High availability](#high-availability)
- [Uninstalling](#uninstalling)
<!-- /toc -->

## Charts

The installation of `mariadb-operator` is splitted into two different helm charts for better convenience:
- [`mariadb-operator-crds`](../deploy/charts/mariadb-operator-crds/): Bundles the [`CustomResourceDefinitions`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) required by the operator.
- [`mariadb-operator`](../deploy/charts/mariadb-operator/): Contains all the template manifests required to install the operator.

## Control-plane

The `mariadb-operator` is an extension of the Kubernetes control-plane, consisting of the following components that are deployed by Helm:
 
- `operator`: The `mariadb-operator` itself that performs the CRD reconciliation.
- `webhook`: The Kubernetes control-plane delegates CRD validations to this HTTP server. Kubernetes requires TLS to communicate with the webhook server.
- `cert-controller`: Provisions TLS certificates for the webhook. You can see it as a minimal [cert-manager](https://cert-manager.io/) that is intended to work only with the webhook. It is optional and can be replaced by cert-manager.

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

If you have the [prometheus operator](https://github.com/prometheus-operator/prometheus-operator) and [cert-manager](https://cert-manager.io/docs/installation/) already installed in your cluster, it is recommended to leverage them to scrape the operator metrics and provision the webhook certificate respectively:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator \
  --set metrics.enabled=true --set webhook.cert.certManager.enabled=true
```

Refer to the helm chart README for detailed information about all the supported [helm values](./../deploy/charts/mariadb-operator/README.md).


## Deployment modes

Deployments are highly configurable via the [helm values](./../deploy/charts/mariadb-operator/README.md). In particular the following deployment modes are supported:

#### Cluster-wide

The operator watches CRDs in all namespaces and requires cluster-wide RBAC permissions to operate. This is the default deployment mode, enabled through the default configuration values:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

#### Single namespace

By setting `currentNamespaceOnly=true`, the operator will only watch CRDs within the namespace it is deployed in, and the RBAC permissions will be restricted to that namespace as well:

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

## High availability

The operator can run in high availability mode to ensure that your CRs get reconciled even if the node where the operator runs goes down. For achieving this you need:
- Multiple replicas
- Configure `Pod` anti-affinity
- Configure `PodDisruptionBudgets` 

You can achieve this by providing the following values to the helm chart:

```yaml
ha:
  enabled: true
  replicas: 3

affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app.kubernetes.io/name
          operator: In
          values:
          - mariadb-operator
        - key: app.kubernetes.io/instance
          operator: In
          values:
          - mariadb-operator
      topologyKey: kubernetes.io/hostname

pdb:
  enabled: true
  maxUnavailable: 1
```

## Uninstalling

> [!CAUTION]
> Uninstalling the `mariadb-operator-crds` Helm chart will remove the CRDs and their associated resources, resulting in downtime.

First, uninstall the `mariadb-operator` Helm chart. This action will not delete your CRDs, so your operands (i.e. `MariaDB` and `MaxScale`) will continue to run without the operator's reconciliation.

```bash
helm uninstall mariadb-operator
```

At this point, if you also want to delete CRDs and the operands running in your cluster, you may proceed to uninstall the `mariadb-operator-crds` Helm chart:

```bash
helm uninstall mariadb-operator-crds
```