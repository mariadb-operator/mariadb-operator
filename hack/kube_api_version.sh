#!/bin/bash

set -eo pipefail

if [[ -z "$KUBE_API_VERSION" ]]; then
  echo "Error: KUBE_API_VERSION environment variable is not set."
  echo "Usage: KUBE_API_VERSION=\"v1.32\" $0"
  exit 1
fi

echo "Updating API doc references to version: $KUBE_API_VERSION"
ESCAPED_VERSION=$(echo "$KUBE_API_VERSION" | sed 's/\./\\./g')
find api/ -type f | while read -r file; do
  if grep -q "kubernetes.io/docs/reference/generated/kubernetes-api" "$file"; then
    sed -i -E "s|(https://kubernetes.io/docs/reference/generated/kubernetes-api)/v[0-9]+\.[0-9]+|\1/${ESCAPED_VERSION}|g" "$file"
  fi
done

echo "API docs processed."