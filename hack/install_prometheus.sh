#!/bin/bash

set -eo pipefail

CONFIG="$( dirname "${BASH_SOURCE[0]}" )"/config
if [ -z "$PROMETHEUS_VERSION" ]; then 
  echo "PROMETHEUS_VERSION environment variable is mandatory"
  exit 1
fi

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update prometheus-community

CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix.go)

helm upgrade --install \
  --version $PROMETHEUS_VERSION \
  -n kube-prometheus-stack --create-namespace \
  -f $CONFIG/kube-prometheus-stack.yaml \
  --set prometheus.service.type=LoadBalancer \
  --set prometheus.service.port=9090 \
  --set prometheus.service.annotations."metallb\.universe\.tf/loadBalancerIPs"=${CIDR_PREFIX}.0.190 \
  --set grafana.service.type=LoadBalancer \
  --set grafana.service.port=3000 \
  --set grafana.service.annotations."metallb\.universe\.tf/loadBalancerIPs"=${CIDR_PREFIX}.0.191 \
  kube-prometheus-stack prometheus-community/kube-prometheus-stack