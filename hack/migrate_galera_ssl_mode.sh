#!/bin/bash

set -eo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <mariadb-instance> <ssl-mode>"
  exit 1
fi

MARIADB_INSTANCE="$1"
SSL_MODE="$2"

for pod in $(kubectl get pods -l app.kubernetes.io/instance="$MARIADB_INSTANCE",app.kubernetes.io/name=mariadb -o jsonpath='{.items[*].metadata.name}'); do
  echo "Updating ssl_mode to $SSL_MODE on pod: $pod"
  kubectl exec -it "$pod" -c mariadb -- sed -i "s/^ssl_mode=.*/ssl_mode=$SSL_MODE/" /etc/mysql/mariadb.conf.d/0-galera.cnf
  echo "Updated $pod successfully"
done
