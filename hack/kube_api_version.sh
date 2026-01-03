#!/bin/bash

set -eo pipefail

if [[ -z "$KUBE_API_VERSION" ]]; then
  echo "Error: KUBE_API_VERSION environment variable is not set."
  echo "Usage: KUBE_API_VERSION=\"v1.32\" $0"
  exit 1
fi

echo "Updating API doc references to version: $KUBE_API_VERSION"
ESCAPED_VERSION=$(echo "$KUBE_API_VERSION" | sed 's/\./\\./g')
# Detect GNU vs BSD sed (sed --version only works in GNU)
if sed --version >/dev/null 2>&1; then
  SED_INPLACE=(-i)
else
  SED_INPLACE=(-i '')
fi
find api/ -type f | while read -r file; do
  if grep -q "kubernetes.io/docs/reference/generated/kubernetes-api" "$file"; then
    sed -E "${SED_INPLACE[@]}" "s|(https://kubernetes.io/docs/reference/generated/kubernetes-api)/v[0-9]+\.[0-9]+|\1/${ESCAPED_VERSION}|g" "$file"
  fi
done

echo "API docs processed."
