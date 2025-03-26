#!/bin/bash

set -eo pipefail

if [[ -z "$MARIADB_NAME" || -z "$MARIADB_NAMESPACE" || -z "$MARIADB_ROOT_PASSWORD" ]]; then
  echo "Error: MARIADB_NAME, MARIADB_NAMESPACE and MARIADB_ROOT_PASSWORD env vars must be set."
  exit 1
fi

function exec_sql {
  local pod=$1
  local sql=$2
  kubectl exec -n "$MARIADB_NAMESPACE" "$pod" -- mariadb -u root -p"$MARIADB_ROOT_PASSWORD" -e "$sql"
}

function wait_for_ready_replication {
  local pod=$1
  local timeout=300  # 5 minutes
  local interval=10  # Check every 10 seconds
  local elapsed=0

  echo "Waiting for ready replication on $pod..."

  while [[ $elapsed -lt $timeout ]]; do
    local status
    status=$(exec_sql "$pod" "SHOW REPLICA STATUS\G" | tee /tmp/replication_status_$pod.txt)

    if grep -q "Slave_IO_Running: Yes" /tmp/replication_status_$pod.txt && \
       grep -q "Slave_SQL_Running: Yes" /tmp/replication_status_$pod.txt; then
      echo "Replication is ready on $pod."
      return 0
    fi

    echo "Replication not ready on $pod. Retrying in $interval seconds..."
    sleep $interval
    ((elapsed+=interval))
  done

  echo "Error: Replication did not become ready on $pod within 5 minutes."
  exit 1
}

echo "Migrating replication on $MARIADB_NAME instance..."

PODS=$(kubectl get pods -n "$MARIADB_NAMESPACE" -l app.kubernetes.io/instance=$MARIADB_NAME -o jsonpath="{.items[*].metadata.name}")
PRIMARY_POD=$(kubectl get mariadb "$MARIADB_NAME" -n "$MARIADB_NAMESPACE" -o jsonpath="{.status.currentPrimary}")
echo "Primary pod detected: $PRIMARY_POD"
echo "Replica pods: $PODS"

for POD in $PODS; do
  if [[ "$POD" == "$PRIMARY_POD" ]]; then
    printf "Skipping primary pod: $POD\n"
    continue
  fi
  printf "Processing replica pod: $POD\n\n"

  echo "Resetting replication on $POD..."
  exec_sql "$POD" "STOP SLAVE 'mariadb-operator';"
  exec_sql "$POD" "RESET SLAVE 'mariadb-operator' ALL;"

  echo "Deleting pod $POD..."
  kubectl delete pod "$POD" -n "$MARIADB_NAMESPACE"

  echo "Waiting for pod $POD to become ready..."
  kubectl wait --for=condition=Ready pod/"$POD" -n "$MARIADB_NAMESPACE" --timeout=5m
  echo "Pod $POD is ready."

  wait_for_ready_replication "$POD"
done

echo "Replication migration completed successfully on $MARIADB_NAME instance."
