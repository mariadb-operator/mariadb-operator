#!/bin/bash

set -eo pipefail

CONFIG="$( dirname "${BASH_SOURCE[0]}" )"/config
if [ -z "$MINIO_VERSION" ]; then 
  echo "MINIO_VERSION environment variable is mandatory"
  exit 1
fi

if [ -z "$HELM" ]; then
  echo "HELM environment variable is mandatory"
  exit 1
fi

${HELM} repo add minio https://charts.min.io/
${HELM} repo update minio

CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix/main.go)

${HELM} upgrade --install \
  --version $MINIO_VERSION \
  -n minio --create-namespace \
  -f $CONFIG/minio.yaml \
  --set service.type=LoadBalancer \
  --set service.annotations."metallb\.universe\.tf/loadBalancerIPs"=${CIDR_PREFIX}.0.200 \
  --set consoleService.type=LoadBalancer \
  --set consoleService.annotations."metallb\.universe\.tf/loadBalancerIPs"=${CIDR_PREFIX}.0.201 \
  minio minio/minio