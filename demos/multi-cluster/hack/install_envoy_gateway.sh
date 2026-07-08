#!/usr/bin/env bash

set -eo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
MANIFESTS="$CURDIR/../manifests/envoy"
ROOT_DIR="$CURDIR/../../.."

if [ -z "$ENVOY_GATEWAY_VERSION" ]; then
  echo "ENVOY_GATEWAY_VERSION environment variable is mandatory"
  exit 1
fi

if [ -z "$HELM" ]; then
  echo "HELM environment variable is mandatory"
  exit 1
fi

# CRDs are installed via helm template | kubectl apply --server-side to avoid
# Helm's 1MB secret size limit. See: https://gateway.envoyproxy.io/docs/install/install-helm/#installing-crds-separately
${HELM} template envoy-gateway-crds \
  oci://docker.io/envoyproxy/gateway-crds-helm \
  --version ${ENVOY_GATEWAY_VERSION} \
  --set crds.gatewayAPI.enabled=true \
  --set crds.gatewayAPI.channel=experimental \
  --set crds.envoyGateway.enabled=true \
  | kubectl apply --server-side -f -

${HELM} upgrade --install envoy-gateway \
  oci://docker.io/envoyproxy/gateway-helm \
  --version ${ENVOY_GATEWAY_VERSION} \
  -n envoy-gateway-system --create-namespace \
  --skip-crds \
  --set config.envoyGateway.extensionApis.enableBackend=true

kubectl apply -f $MANIFESTS/envoy-proxy.yaml
kubectl apply -f $MANIFESTS/gateway.yaml

kubectl wait --for=condition=Programmed gateway/mariadb-gateway \
  -n envoy-gateway-system --timeout=60s