#!/bin/bash

if [ -z "$CERT_MANAGER_VERSION" ]; then
  CERT_MANAGER_VERSION="v1.9.1"
fi

helm repo add jetstack https://charts.jetstack.io
helm repo update
helm upgrade --install \
  --version $CERT_MANAGER_VERSION \
  -n cert-manager --create-namespace \
  --set installCRDs=true \
  --set prometheus.enabled=false \
  cert-manager jetstack/cert-manager
