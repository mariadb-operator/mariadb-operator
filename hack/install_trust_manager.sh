#!/bin/bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG="$CURDIR/config"
MANIFESTS="$CURDIR/manifests/trust-manager"
if [ -z "$TRUST_MANAGER_VERSION" ]; then
  echo "TRUST_MANAGER_VERSION environment variable is mandatory"
  exit 1
fi

helm repo add jetstack https://charts.jetstack.io
helm repo update jetstack

helm upgrade --install \
  --version $TRUST_MANAGER_VERSION \
  -n trust-manager --create-namespace \
  -f $CONFIG/trust-manager.yaml \
  trust-manager jetstack/trust-manager

kubectl apply -f "$MANIFESTS/bundle.yaml"