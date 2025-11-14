#!/bin/bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG="$CURDIR/config"
MANIFESTS="$CURDIR/manifests/trust-manager"
if [ -z "$TRUST_MANAGER_VERSION" ]; then
  echo "TRUST_MANAGER_VERSION environment variable is mandatory"
  exit 1
fi

if [ -z "$HELM" ]; then
  echo "HELM environment variable is mandatory"
  exit 1
fi

${HELM} repo add jetstack https://charts.jetstack.io
${HELM} repo update jetstack

${HELM} upgrade --install \
  --version $TRUST_MANAGER_VERSION \
  -n trust-manager --create-namespace \
  -f $CONFIG/trust-manager.yaml \
  trust-manager jetstack/trust-manager
kubectl wait -n trust-manager  \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/name=trust-manager  \
  --timeout=120s

kubectl apply -f "$MANIFESTS/bundle.yaml"