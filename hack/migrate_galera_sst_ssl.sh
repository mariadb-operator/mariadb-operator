#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <mariadb-instance>"
  exit 1
fi

MARIADB_INSTANCE="$1"

for pod in $(kubectl get pods -l app.kubernetes.io/instance="$MARIADB_INSTANCE",app.kubernetes.io/name=mariadb -o jsonpath='{.items[*].metadata.name}'); do
  echo "Migrating Pod $pod"
  echo "Enabling SSL for Galera SST"
  kubectl exec -it "$pod" -c mariadb -- sh -c "cat << EOF >> /etc/mysql/mariadb.conf.d/0-galera.cnf
[sst]
encrypt=3
tca=/etc/pki/ca.crt
tcert=/etc/pki/client.crt
tkey=/etc/pki/client.key
EOF"
  echo "Pod $pod migrated successfully"
done
