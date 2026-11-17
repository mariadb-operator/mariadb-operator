#!/bin/bash

set -euo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

kubectl apply -f $CURDIR/manifests/serviceaccount.yaml
TOKEN_PATH="${MARIADB_OPERATOR_SA_PATH:-/tmp/mariadb-operator/token}"
mkdir -p "$(dirname "$TOKEN_PATH")"
kubectl get secret mariadb-operator -o jsonpath="{.data.token}" | base64 -d > "$TOKEN_PATH"