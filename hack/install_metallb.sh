#!/usr/bin/env bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
if [ -z "$METALLB_VERSION" ]; then
  echo "METALLB_VERSION environment variable is mandatory"
  exit 1
fi

helm repo add metallb https://metallb.github.io/metallb
helm repo update metallb

helm upgrade --install \
  --version $METALLB_VERSION \
  -n metallb --create-namespace \
  metallb metallb/metallb
kubectl wait -n metallb \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/name=metallb \
  --timeout=120s

export CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix.go)

for f in $CURDIR/manifests/metallb/*; 
do 
  cat $f | envsubst | kubectl apply -f -; 
done;