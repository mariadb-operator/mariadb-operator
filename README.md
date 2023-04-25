<p align="center">
<img src="https://mariadb-operator.github.io/mariadb-operator/assets/mariadb-operator.png" alt="mariadb" width="250"/>
</p>

<p align="center">
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/ci.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/helm.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/helm.yml/badge.svg" alt="Helm"></a>
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/release.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/release.yml/badge.svg" alt="Release"></a>
<a href="https://goreportcard.com/report/github.com/mariadb-operator/mariadb-operator"><img src="https://goreportcard.com/badge/github.com/mariadb-operator/mariadb-operator" alt="Go Report Card"></a>
<a href="https://pkg.go.dev/github.com/mariadb-operator/mariadb-operator"><img src="https://pkg.go.dev/badge/github.com/mariadb-operator/mariadb-operator.svg" alt="Go Reference"></a>
</p>

<p align="center">
<a href="https://artifacthub.io/packages/helm/mariadb-operator/mariadb-operator"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/mariadb-operator" alt="Artifact Hub"></a>
<a href="https://operatorhub.io/operator/mariadb-operator"><img src="https://img.shields.io/badge/Operator%20Hub-mariadb--operator-red" alt="Operator Hub"></a>
</p>

# 🦭 mariadb-operator

Run and operate MariaDB in a cloud native way. Declaratively manage your MariaDB using Kubernetes [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) rather than imperative commands.
- [Provisioning](./config/samples/mariadb_v1alpha1_mariadb.yaml) highly configurable MariaDB servers.
- High availability via SemiSync [replication](./config/samples/mariadb_v1alpha1_mariadb_replication.yaml). Primary switchover
- [Take](./config/samples/mariadb_v1alpha1_backup.yaml) and [restore](./config/samples/mariadb_v1alpha1_restore.yaml) backups. [Scheduled](./config/samples/mariadb_v1alpha1_backup_scheduled.yaml) backups. Backup rotation
- Bootstrap new instances from [backups](./config/samples/mariadb_v1alpha1_mariadb_from_backup.yaml) and volumes ([PVCs](./config/samples/mariadb_v1alpha1_mariadb_from_pvc.yaml), [NFS](./config/samples/mariadb_v1alpha1_mariadb_from_nfs.yaml) ...)
- Manage [users](./config/samples/mariadb_v1alpha1_user.yaml), [grants](./config/samples/mariadb_v1alpha1_grant.yaml) and logical [databases](./config/samples/mariadb_v1alpha1_database.yaml)
- Configure [connections](./config/samples/mariadb_v1alpha1_connection.yaml) for your applications
- Orchestrate [sql scripts](./config/samples/sqljobs)
- Prometheus metrics
- Validation webhooks to provide CRD inmutability
- Additional printer columns to report the current CRD status
- CRDs designed according to the Kubernetes [API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [GitOps](https://opengitops.dev/) friendly
- Multi-arch [docker](https://github.com/orgs/mariadb-operator/packages/container/package/mariadb-operator) image
- Install it using [kubectl](./deploy/manifests), [helm](https://artifacthub.io/packages/helm/mariadb-operator/mariadb-operator) or [OLM](https://operatorhub.io/operator/mariadb-operator) 

## Bare minimum installation

This installation flavour provides the minimum resources required to run `mariadb-operator` in your cluster.

```bash
helm repo add mariadb-operator https://mariadb-operator.github.io/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```

## Recommended installation

The recommended installation includes the following features to provide a better user experiende and reliability:
- **Metrics**: Leverage [prometheus operator](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) to scrape metrics from both the `mariadb-operator` and the provisioned `MariaDB` instances.
- **Webhook certificate renewal**: Automatic webhook certificate issuance and renewal using  [cert-manager](https://cert-manager.io/docs/installation/). By default, a static self-signed certificate is generated.

```bash
helm repo add mariadb-operator https://mariadb-operator.github.io/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator \
  --set metrics.enabled=true --set webhook.certificate.certManager=true
```

## Openshift

The Openshift installation is managed separately in the [mariadb-operator-helm](https://github.com/mariadb-operator/mariadb-operator-helm) repository, which contains a [helm based operator](https://sdk.operatorframework.io/docs/building-operators/helm/) that allows you to install `mariadb-operator` via [OLM](https://olm.operatorframework.io/docs/).

## Quickstart

Let's see `mariadb-operator`🦭 in action! First of all, install the following configuration manifests that will be referenced by the CRDs further:
```bash
kubectl apply -f config/samples/config
```

To start with, let's provision a `MariaDB` instance:
```bash
kubectl apply -f config/samples/mariadb_v1alpha1_mariadb.yaml
```
```bash
kubectl get mariadbs
NAME      READY   STATUS    PRIMARY     AGE
mariadb   True    Running   mariadb-0   3m57s

kubectl get statefulsets
NAME      READY   AGE
mariadb   1/1     2m12s

kubectl get services
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
mariadb      ClusterIP   10.96.235.145   <none>        3306/TCP,9104/TCP   2m17s
```
Up and running 🚀, we can now create our first logical database and grant access to users:
```bash
kubectl apply -f config/samples/mariadb_v1alpha1_database.yaml
kubectl apply -f config/samples/mariadb_v1alpha1_user.yaml
kubectl apply -f config/samples/mariadb_v1alpha1_grant.yaml
```
```bash
kubectl get databases
NAME        READY   STATUS    CHARSET   COLLATE           AGE
data-test   True    Created   utf8      utf8_general_ci   22s

kubectl get users
NAME              READY   STATUS    MAXCONNS   AGE
mariadb-metrics   True    Created   3          19m
user              True    Created   20         29s

kubectl get grants
NAME              READY   STATUS    DATABASE   TABLE   USERNAME          GRANTOPT   AGE
mariadb-metrics   True    Created   *          *       mariadb-metrics   false      19m
user              True    Created   *          *       user              true       36s
```
At this point, we can run our database initialization scripts:
```bash
kubectl apply -f config/samples/sqljobs
```
```bash
kubectl get sqljobs
NAME                      COMPLETE   STATUS    MARIADB   AGE
01-create-table-users     True       Success   mariadb   5m34s
02-1-insert-users         True       Success   mariadb   5m34s
02-2-create-table-repos   True       Success   mariadb   5m34s
03-insert-repos           True       Success   mariadb   5m34s

kubectl get jobs
NAME                      COMPLETIONS   DURATION   AGE
01-create-table-users     1/1           5s         5m51s
02-1-insert-users         1/1           5s         5m45s
02-2-create-table-repos   1/1           5s         5m45s
03-insert-repos           1/1           6s         5m39s
```

Now that the database has been initialized, let's take a backup:
```bash
kubectl apply -f config/samples/mariadb_v1alpha1_backup_scheduled.yaml
```
After one minute, the backup should have completed:
```bash
kubectl get backups
NAME               COMPLETE   STATUS    MARIADB   AGE
backup-scheduled   True       Success   mariadb   15m

kubectl get cronjobs
NAME               SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
backup-scheduled   */1 * * * *   False     0        56s             15m

kubectl get jobs
NAME                                    COMPLETIONS   DURATION   AGE
backup-scheduled-27782894               1/1           4s         3m2s
```
Last but not least, let's provision a second `MariaDB` instance bootstrapping from the previous backup:
```bash
kubectl apply -f config/samples/mariadb_v1alpha1_mariadb_from_backup.yaml
``` 
```bash
kubectl get mariadbs
NAME                       READY   STATUS    PRIMARY     AGE
mariadb                    True    Running   mariadb-0   30m57s
mariadb-from-backup        True    Running   mariadb-0   85s

kubectl get restores
NAME                                         COMPLETE   STATUS    MARIADB               AGE
bootstrap-restore-mariadb-from-backup        True       Success   mariadb-from-backup   72s

kubectl get jobs
NAME                                         COMPLETIONS   DURATION   AGE
backup                                       1/1           9s         12m
bootstrap-restore-mariadb-from-backup        1/1           5s         84s
``` 
You can take a look at the whole suite of example CRDs available in [config/samples](./config/samples/).

## Advanced examples

More advance scenarios can be found under the [examples](./examples/) folder:
- [Run and operate MariaDB in a GitOps fashion using Flux](./examples/flux/)

## Roadmap

Take a look at our [🛣️ roadmap](./ROADMAP.md) and feel free to open an issue to suggest new features.


## Contributing

If you want to report a 🐛 or you think something can be improved, please check our [contributing](./CONTRIBUTING.md) guide and take a look at our open [issues](https://github.com/mariadb-operator/mariadb-operator/issues). PRs are welcome!
