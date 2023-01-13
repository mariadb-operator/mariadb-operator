# 🦭 mariadb-operator
[![CI](https://github.com/mmontes11/mariadb-operator/actions/workflows/ci.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/ci.yml)
[![Helm](https://github.com/mmontes11/mariadb-operator/actions/workflows/helm.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/helm.yml)
[![Release](https://github.com/mmontes11/mariadb-operator/actions/workflows/release.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mmontes11/mariadb-operator)](https://goreportcard.com/report/github.com/mmontes11/mariadb-operator)
[![Go Reference](https://pkg.go.dev/badge/github.com/mmontes11/mariadb-operator.svg)](https://pkg.go.dev/github.com/mmontes11/mariadb-operator)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/mariadb-operator)](https://artifacthub.io/packages/helm/mariadb-operator/mariadb-operator)

Run and operate MariaDB in a cloud native way. Declaratively manage your MariaDB using Kubernetes [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) rather than imperative commands.

- [Provisioning](./config/samples/database_v1alpha1_mariadb.yaml) highly configurable MariaDB servers
- [Take](./config/samples/database_v1alpha1_backupmariadb.yaml) and [restore](./config/samples/database_v1alpha1_restoremariadb.yaml) backups. [Scheduled](./config/samples/database_v1alpha1_backupmariadb_scheduled.yaml) backups. Backup rotation
- Bootstrap new instances from [backups](./config/samples/database_v1alpha1_mariadb_from_backup.yaml) and volumes ([PVCs](./config/samples/database_v1alpha1_mariadb_from_pvc.yaml), [NFS](./config/samples/database_v1alpha1_mariadb_from_nfs.yaml) ...)
- Support for managing [users](./config/samples/database_v1alpha1_usermariadb.yaml), [grants](./config/samples/database_v1alpha1_grantmariadb.yaml) and logical [databases](./config/samples/database_v1alpha1_databasemariadb.yaml)
- Prometheus metrics
- Validation webhooks to provide CRD inmutability
- Additional printer columns to report the current CRD status
- CRDs designed according to the Kubernetes [API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [GitOps](https://opengitops.dev/) friendly
- Multi-arch [docker](https://hub.docker.com/repository/docker/mmontes11/mariadb-operator/tags?page=1&ordering=last_updated) image
- Install using [helm](./deploy/charts/mariadb-operator) or just [kubectl](./deploy/manifests)

## Bare minimum installation

This installation flavour provides the minimum resources required to run mariadb operator. You can install it using the helm chart:

```bash
helm repo add mariadb-operator https://mmontes11.github.io/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator
```
or alternatively just kubectl apply the manifests bundle:

```bash
MDB_VERSION=v0.0.5
kubectl apply -f https://github.com/mmontes11/mariadb-operator/releases/download/$MDB_VERSION/manifests.min.yaml
```


## Recommended installation

The recommended installation includes the following features to provide a better user experiende and reliability:
- **Validation WebHooks**: To ensure resource inmutability and provide more accurate validations. To enable this feature, [cert-manager](https://cert-manager.io/docs/installation/) should be installed in the cluster in order to reconcile `Certificate` resources for the webhooks.
- **Metrics**: To provide observability in both the mariadb controller and the provisioned MariaDB instances. To enable this feature, [prometheus operator](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) should be present in the cluster to reconcile `ServiceMonitor` resources.

```bash
helm repo add mariadb-operator https://mmontes11.github.io/mariadb-operator
helm install mariadb-operator mariadb-operator/mariadb-operator \
  --set webhook.enabled=true --set metrics.enabled=true 
```

```bash
MDB_VERSION=v0.0.5
kubectl apply -f https://github.com/mmontes11/mariadb-operator/releases/download/$MDB_VERSION/manifests.yaml
```

## Quickstart

Let's see `mariadb-operator`🦭 in action! First of all, install the following configuration manifests that will be referenced by the CRDs further:
```bash
kubectl apply -f config/samples/config
```

To start with, let's provision a `MariaDB` server with Prometheus metrics:
```bash
kubectl apply -f config/samples/database_v1alpha1_mariadb.yaml
```
```bash
kubectl get mariadbs
NAME      READY   STATUS    AGE
mariadb   True    Running   75s

kubectl get statefulsets
NAME      READY   AGE
mariadb   1/1     2m12s

kubectl get services
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
mariadb      ClusterIP   10.96.235.145   <none>        3306/TCP,9104/TCP   2m17s

kubectl get servicemonitors
NAME      AGE
mariadb   2m37s
```
Up and running 🚀, we can now create our first logical database and grant access to users:
```bash
kubectl apply -f config/samples/database_v1alpha1_databasemariadb.yaml
kubectl apply -f config/samples/database_v1alpha1_usermariadb.yaml
kubectl apply -f config/samples/database_v1alpha1_grantmariadb.yaml
```
```bash
kubectl get databasemariadbs
NAME        READY   STATUS    CHARSET   COLLATE           AGE
data-test   True    Created   utf8      utf8_general_ci   22s

kubectl get usermariadbs
NAME              READY   STATUS    MAXCONNS   AGE
mariadb-metrics   True    Created   3          19m
user              True    Created   20         29s

kubectl get grantmariadbs
NAME              READY   STATUS    DATABASE   TABLE   USERNAME          GRANTOPT   AGE
mariadb-metrics   True    Created   *          *       mariadb-metrics   false      19m
user              True    Created   *          *       user              true       36s
```
Now that everything seems to be in place, let's take a backup:
```bash
kubectl apply -f config/samples/database_v1alpha1_backupmariadb_scheduled.yaml
```
After one minute, the backup should have completed:
```bash
kubectl get backupmariadbs
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
kubectl apply -f config/samples/database_v1alpha1_mariadb_from_backup.yaml
``` 
```bash
kubectl get mariadbs
NAME                       READY   STATUS    AGE
mariadb                    True    Running   39m
mariadb-from-backup        True    Running   85s

kubectl get restoremariadbs
NAME                                         COMPLETE   STATUS    MARIADB               AGE
bootstrap-restore-mariadb-from-backup        True       Success   mariadb-from-backup   72s

kubectl get jobs
NAME                                         COMPLETIONS   DURATION   AGE
backup                                       1/1           9s         12m
bootstrap-restore-mariadb-from-backup        1/1           5s         84s
``` 
You can take a look at the whole suite of example CRDs available in [config/samples](./config/samples/).  

## Roadmap

Take a look at our [🛣️ roadmap](./ROADMAP.md) and feel free to open an issue to suggest new features.


## Contributing

If you want to report a 🐛 or you think something can be improved, please check our [contributing](./CONTRIBUTING.md) guide and take a look at our open [issues](https://github.com/mmontes11/mariadb-operator/issues). PRs are welcome!