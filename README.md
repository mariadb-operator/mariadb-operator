# 🦭 mariadb-operator
[![CI](https://github.com/mmontes11/mariadb-operator/actions/workflows/ci.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/ci.yml)
[![Release](https://github.com/mmontes11/mariadb-operator/actions/workflows/release.yml/badge.svg)](https://github.com/mmontes11/mariadb-operator/actions/workflows/release.yml)

Run and operate MariaDB in a cloud native way.

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
- Install `mariadb-operator`
```bash
helm repo add mmontes https://charts.mmontes-dev.duckdns.org
helm repo update
helm install mariadb-operator mmontes/mariadb-operator \
  -n mariadb-system --create-namespace
``` 