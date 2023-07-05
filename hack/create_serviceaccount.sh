#!/bin/bash

set -euo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

kubectl apply -f $CURDIR/manifests/serviceaccount.yaml
mkdir -p $CURDIR/../mariadb-operator
kubectl get secret mariadb-operator -o jsonpath="{.data.token}" | base64 -d > $CURDIR/../mariadb-operator/token