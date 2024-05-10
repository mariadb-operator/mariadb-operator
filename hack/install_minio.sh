#!/bin/bash

set -eo pipefail

CONFIG="$( dirname "${BASH_SOURCE[0]}" )"/config
if [ -z "$MINIO_VERSION" ]; then 
  echo "MINIO_VERSION environment variable is mandatory"
  exit 1
fi

helm repo add minio https://charts.min.io/
helm repo update minio

CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix.go)

helm upgrade --install \
  --version $MINIO_VERSION \
  -n minio --create-namespace \
  -f $CONFIG/minio.yaml \
  --set service.type=LoadBalancer \
  --set service.annotations."metallb\.universe\.tf/loadBalancerIPs"=${CIDR_PREFIX}.0.200 \
  --set consoleService.type=LoadBalancer \
  --set consoleService.annotations."metallb\.universe\.tf/loadBalancerIPs"=${CIDR_PREFIX}.0.201 \
  minio minio/minio