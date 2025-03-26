# Upgrade guide 0.38.0

This guide illustrates, step by step, how to migrate to `0.38.0` from previous versions. 

> [!NOTE]  
> Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version.

- Uninstall you current `mariadb-operator` for preventing conflicts:
```bash
helm uninstall mariadb-operator
```
Alternatively, you may only downscale and delete the webhook configurations:
```bash
kubectl scale deployment mariadb-operator --replicas=0
kubectl scale deployment mariadb-operator-webhook --replicas=0
kubectl delete validatingwebhookconfiguration mariadb-operator-webhook
kubectl delete mutatingwebhookconfiguration mariadb-operator-webhook
```

- Upgrade `mariadb-operator-crds` to `0.38.0`:

```bash
helm repo update mariadb-operator
helm upgrade --install mariadb-operator-crds  mariadb-operator/mariadb-operator-crds --version 0.38.0
```

- If you are using Galera, and you want the operator to automatically update the data-plane (i.e. init and agent containers) to `0.38.0`, you can set `updateStrategy.autoUpdateDataPlane=true` in your `MariaDB` resources:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   autoUpdateDataPlane: true
```

Alternatively, you can also do this manually:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  galera:
    agent:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.1
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.38.0
    initContainer:
-      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.37.1
+      image: docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:0.38.0
```

-  Upgrade `mariadb-operator` to `0.38.0`:
```bash 
helm repo update mariadb-operator
helm upgrade --install mariadb-operator mariadb-operator/mariadb-operator --version 0.38.0 
```

- If you previously decided to downscale the operator, make sure you upscale it back:
```bash
kubectl scale deployment mariadb-operator --replicas=1
kubectl scale deployment mariadb-operator-webhook --replicas=1
```

- If you previously set `updateStratety.autoUpdateDataPlane=true`, you may consider reverting the changes once the upgrades have finished:

```diff
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  updateStrategy:
+   autoUpdateDataPlane: false
-   autoUpdateDataPlane: true
```

- If you are using asynchronous replication, it is important that you understand the changes described in https://github.com/mariadb-operator/mariadb-operator/pull/1219. In order to migrate, you can use the following script:

```bash
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
    status=$(exec_sql "$pod" "SHOW REPLICA STATUS\G" | tee /tmp/replication_status_$pod_$MARIADB_NAMESPACE.txt)

    if grep -q "Slave_IO_Running: Yes" /tmp/replication_status_$pod_$MARIADB_NAMESPACE.txt && \
       grep -q "Slave_SQL_Running: Yes" /tmp/replication_status_$pod_$MARIADB_NAMESPACE.txt; then
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

for POD in $PODS; do
  if [[ "$POD" == "$PRIMARY_POD" ]]; then
    printf "\nSkipping primary pod: $POD\n"
    continue
  fi
  printf "\nProcessing replica pod: $POD\n"

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
```

For better convenience, replace the variables and run:

```bash
curl -sLO https://raw.githubusercontent.com/mariadb-operator/mariadb-operator/refs/heads/main/hack/migrate_repl_conn.sh
chmod +x migrate_repl_conn.sh

MARIADB_NAME='<mariadb-name>' \
MARIADB_NAMESPACE='<mariadb-namespace>' \
MARIADB_ROOT_PASSWORD='<mariadb-root-password>' \
./migrate_repl_conn.sh
```