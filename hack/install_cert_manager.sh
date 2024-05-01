#!/bin/bash

set -eo pipefail

CONFIG="$( dirname "${BASH_SOURCE[0]}" )"/config
if [ -z "$CERT_MANAGER_VERSION" ]; then
  echo "CERT_MANAGER_VERSION environment variable is mandatory"
  exit 1
fi

helm repo add jetstack https://charts.jetstack.io
helm repo update jetstack

helm upgrade --install \
  --version $CERT_MANAGER_VERSION \
  -n cert-manager --create-namespace \
  -f $CONFIG/cert-manager.yaml \
  cert-manager jetstack/cert-manager
