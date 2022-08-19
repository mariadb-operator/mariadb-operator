# 🦭 mariadb-operator
[![CI](https://github.com/mmontes11/mariadb-operator/actions/workflows/ci.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/ci.yml)
[![Release](https://github.com/mmontes11/mariadb-operator/actions/workflows/release.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/release.yml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/mmontes)](https://artifacthub.io/packages/search?repo=mmontes)

Run and operate MariaDB in a cloud native way. Declaratively manage your MariaDB using Kubernetes [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) rather than imperative commands.

- MariaDB server provisioning
- Seamless upgrades without data loss
- Take and restore backups
- Bootstrap new instances from a backup
- Support for managing users, grants and logical databases
- Prometheus metrics
- Validation wehhooks to provide CRD inmutability
- [GitOps](https://opengitops.dev/) friendly
- CRDs designed according to the Kubernetes [API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- Multi-arch [Docker](https://hub.docker.com/repository/docker/mmontes11/mariadb-operator/tags?page=1&ordering=last_updated) image
- [Helm](./helm/mariadb-operator/) chart 

### Installation

- Install [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -n kube-prometheus-stack --create-namespace
``` 
- Install [cert-manager](https://github.com/cert-manager/cert-manager) 
```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  -n cert-manager --create-namespace \
  --set installCRDs=true 
```
- Install `mariadb-operator` 🦭
```bash
helm repo add mmontes https://charts.mmontes-dev.duckdns.org
helm repo update
helm install mariadb-operator mmontes/mariadb-operator \
  -n mariadb-system --create-namespace
```

### Getting started


### Contributing

Contributions are welcome! If you think something could be improved, request a new feature or just want to leave some feedback,
please check our [contributing](./CONTRIBUTING.md) guide and take a look at our open [issues](https://github.com/mmontes11/mariadb-operator/issues).
