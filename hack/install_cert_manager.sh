#!/bin/bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG="$CURDIR/config"
MANIFESTS="$CURDIR/manifests/cert-manager"
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

kubectl apply -f "$MANIFESTS/selfsigned-clusterissuer.yaml"

kubectl apply -f "$MANIFESTS/root-certificate.yaml"
kubectl wait --for=condition=Ready certificate root-ca --timeout=30s
kubectl apply -f "$MANIFESTS/root-clusterissuer.yaml"
kubectl wait --for=condition=Ready clusterissuer root-ca --timeout=30s

kubectl apply -f "$MANIFESTS/intermediate-certificate.yaml"
kubectl wait --for=condition=Ready certificate intermediate-ca --timeout=30s
kubectl apply -f "$MANIFESTS/intermediate-clusterissuer.yaml"
kubectl wait --for=condition=Ready clusterissuer intermediate-ca --timeout=30s

CUSTOM_TRUST="$(kubectl get secret root-ca -o jsonpath='{.data.tls\.crt}' | base64 -d; kubectl get secret intermediate-ca -o jsonpath='{.data.tls\.crt}' | base64 -d)"
kubectl create secret generic custom-trust --from-literal=ca.crt="$CUSTOM_TRUST" --dry-run=client -o yaml | kubectl apply -f -