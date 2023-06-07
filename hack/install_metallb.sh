#!/usr/bin/env bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
if [ -z "$METALLB_VERSION" ]; then
  METALLB_VERSION="0.13.9"
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
  --timeout=90s
kubectl apply -f $CURDIR/metallb 
