
`{{ .ProjectName }}` __[v0.0.32](https://github.com/mariadb-operator/mariadb-operator/releases/tag/v0.0.32)__ is out! 🦭

This release ships new features and improvements focused on fleet management, upgrades and deployments. Check them out below!

To upgrade from older versions, be sure to follow the __[UPGRADE GUIDE](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/UPGRADE_v0.0.32.md)__.

### Auto update data-plane

Galera relies on [data-plane containers](https://github.com/mariadb-operator/mariadb-operator/tree/main/docs/GALERA.md#data-plane) that run alongside MariaDB to implement provisioning and high availability operations on the cluster. These containers use the `mariadb-operator` image, which can be automatically updated by the operator based on its image version:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
    autoUpdateDataPlane: true
```
By default, `updateStrategy.autoUpdateDataPlane` is `false`, which means that no automatic upgrades will be performed, but you can opt-in/opt-out from this feature at any point in time by updating this field. For instance, you may want to selectively enable `updateStrategy.autoUpdateDataPlane` in a subset of your `MariaDB` instances after the operator has been upgraded to a newer version, and then disable it once the upgrades are completed.

### Pause updates via `Never` update strategy

With this new update strategy set, the operator will `Never` perform updates on the `StatefulSet`. This could be useful in multiple scenarios:
- __Progressive fleet upgrades__: If you're managing large fleets of of databases, you likely prefer to roll out updates progressively rather than simultaneously across all instances.
- __Operator upgrades__: When upgrading `mariadb-operator`, changes to the `StatefulSet` or the `Pod` template may occur from one version to another, which could trigger a rolling update of your `MariaDB` instances.

You can configure this new strategy by setting:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
    type: Never
``` 

It is important to note that this feature is fully compatible with `autoUpdateDataPlane`: no upgrades will happen when `updateStrategy.autoUpdateDataPlane=true` and `updateStrategy.type=Never`.

### New `mariadb-operator-crds` helm chart

Helm has certain [limitations when it comes to manage CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations). To address this, we are providing the CRDs in a separate chart, [as recommended by the official Helm documentation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#method-2-separate-charts). This allows us to manage the installation and updates of the CRDs independently from the operator. For example, you can uninstall the operator without impacting your existing `MariaDB` CRDs.

CRDs can now be installed/upgraded in your cluster by running the following commands

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm upgrade --install mariadb-operator-crds mariadb-operator/mariadb-operator-crds
```

### ~81% CRD size reduction

Have you seen this before?

```bash
Secret "sh.helm.release.v1.x.v1" is invalid: data: Too long: must have at most 1048576 character
```

Helm has a hard limit of 1MB in the size of the releases to be installed. This was a problem for us, since our [CRD bundle was 3.1M](https://github.com/mariadb-operator/mariadb-operator/blob/v0.0.31/deploy/crds/crds.yaml) in previous releases. This made it incompatible with Helm, so the only choice was to use `kubectl apply` directly to upgrade CRDs.

To address this, we have reduced the size of our CRDs by replacing the upstream Kubernetes types, which were used directly in our CRDs, with a more lightweight version of these types that only contain the fields we support. See https://github.com/mariadb-operator/mariadb-operator/pull/869.

Our [CRD bundle is now 580K](https://github.com/mariadb-operator/mariadb-operator/blob/fcdab4bcb297fda0b82aa8b5e0fe22d00563f590/deploy/crds/crds.yaml), an ~81% slimmer than before!  🧹

### Single namespace deployment

### Basic auth support in Galera agent

### Support for `args` in `MariaDB` 

Kudos to @onesolpark for this contribution! 🙏🏻

### Fix ephemeral storage reconcile

Kudos to @Uburro for this contribution! 🙏🏻

---

We value your feedback! If you encounter any issues or have suggestions, please [open an issue on GitHub](https://github.com/mariadb-operator/mariadb-operator/issues/new/choose). Your input is crucial to improve `{{ .ProjectName }}`🦭.

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.