# Multi-cluster demo

This demo sets up three kind clusters running a multi-cluster MariaDB replication topology behind an Envoy Gateway load balancer:

- `eu-south` — the primary cluster.
- `eu-central` — bootstraps from a physical backup stored in MinIO and joins as a replica.
- `envoy` — runs Envoy Gateway, which load balances client traffic (`app.mariadb.com`) towards the active region and is updated to perform the cutover during a regional failover.

## Architecture

![Multi-cluster architecture](slides/multi-cluster.png)

## Prerequisites

- [kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [helm](https://helm.sh/)
- [kubectx](https://github.com/ahmetb/kubectx) (optional, used in demo steps)

## Installation

Run the full setup with a single command:

```bash
make multi-cluster
```

This runs the following targets in order:

| Target | Description |
|---|---|
| `make host` | Add hosts to `/etc/hosts` |
| `make clusters` | Create the `eu-south`, `eu-central` and `envoy` kind clusters |
| `make pki` | Generate CA certificates and install them in all clusters |
| `make config` | Apply common configuration (secrets, etc.) to all clusters |
| `make coredns` | Patch CoreDNS with cross-cluster DNS entries and restart it |
| `make operator` | Deploy the mariadb-operator via OCI Helm charts in both regions |
| `make metallb` | Install MetalLB in all clusters |
| `make minio` | Install MinIO in `eu-south` |
| `make envoy-gateway` | Install Envoy Gateway in the `envoy` cluster |

Each target can also be run individually. Per-cluster variants are available as `<target>-eu-south`, `<target>-eu-central` and `<target>-envoy`.

## Demo

### 1. Deploy the primary in eu-south

Deploy the primary MariaDB instance and schedule physical backups to MinIO:

```bash
kubectx kind-eu-south
kubectl apply -f manifests/eu-south.yaml
kubectl apply -f manifests/eu-south-backup.yaml
```

### 2. Deploy the replica in eu-central

Deploy the replica MariaDB instance, which bootstraps from the latest backup in MinIO and joins the replication topology:

```bash
kubectx kind-eu-central
kubectl apply -f manifests/eu-central.yaml
```

### 3. Register the MariaDB backends in Envoy

Apply the `Backend` and `TCPRoute` resources. Initially all traffic is weighted towards `eu-south`:

```bash
kubectx kind-envoy
kubectl apply -f manifests/envoy/tcproute.yaml
```

### 4. Start the application

Run the sample client, which connects through the load balancer (`app.mariadb.com`) and continuously inserts rows:

```bash
make app
```

### 5. Tear down the eu-south region

Simulate a regional outage by deleting the primary cluster:

```bash
make cluster-delete-eu-south
```

### 6. Promote eu-central to primary

Patch the `MariaDB` resource to promote `eu-central` as the new multi-cluster primary:

```bash
kubectx kind-eu-central
kubectl patch mariadb mariadb-eu-central --type merge \
  -p '{"spec":{"multiCluster":{"primary":"mariadb-eu-central"}}}'
```

### 7. Cut over traffic in Envoy

Flip the backend weights so the load balancer routes all traffic to `eu-central`:

```bash
kubectx kind-envoy
kubectl patch tcproute mariadb -n envoy-gateway-system --type json \
  -p '[{"op":"replace","path":"/spec/rules/0/backendRefs/0/weight","value":0},{"op":"replace","path":"/spec/rules/0/backendRefs/1/weight","value":100}]'
```

The application started in step 4 keeps running throughout the failover and resumes inserting against `eu-central` once the cutover completes.

__We've survived a regional outage! 🦭🚀__

## DNS

CoreDNS is patched in both clusters with the following entries:

| Hostname | IP |
|---|---|
| `app.mariadb.com` | `172.18.1.100` |
| `mariadb-eu-south.mariadb.com` | `172.18.1.10` |
| `mariadb-eu-central.mariadb.com` | `172.18.1.15` |
| `minio.mariadb.com` | `172.18.0.200` |

## Cleanup

```bash
make clusters-delete
```
