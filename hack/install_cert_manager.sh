#!/bin/bash

VERSION="v1.9.1"

helm repo add jetstack https://charts.jetstack.io
helm repo update
helm upgrade --install \
  --version $VERSION \
  -n cert-manager --create-namespace \
  --set installCRDs=true \
  --set prometheus.enabled=false \
  cert-manager jetstack/cert-manager
