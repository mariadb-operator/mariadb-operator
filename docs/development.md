# Development guide

In this guide, we will be configuring a local environment to run `mariadb-operator` so you can develop and test features without hassle. The local `mariadb-operator` will be able to resolve DNS and connect to MariaDB as if it was running inside a Kubernetes cluster.

## Table of contents
<!-- toc -->
- [Flavours](#flavours)
- [Getting started](#getting-started)
- [Running modes](#running-modes)
- [Cluster](#cluster)
- [Network](#network)
- [Dependencies](#dependencies)
- [Generate](#generate)
- [Install](#install)
- [Build](#build)
- [Run](#run)
- [Test](#test)
<!-- /toc -->

## Flavours

#### devcontainer

Spin up a [devcontainer](https://containers.dev/implementors/json_reference/) with everything you need to develop. This can be used in conjunction with many tools, such as [vscode](https://code.visualstudio.com/docs/devcontainers/containers), [GitHub codespaces](https://github.com/features/codespaces) and [DevPod](https://devpod.sh/), which will automatically detect the [devcontainer.json](../.devcontainer/devcontainer.json).

The only dependency you need is [docker](https://www.docker.com/) in case that choose to run your devcontainer locally.

#### local

Run the operator locally in your machine using `go run`. It requires the following dependencies:
- [make](https://www.gnu.org/software/make/manual/make.html)
- [go](https://go.dev/doc/install)
- [docker](https://www.docker.com/)

This flavour uses [KIND](https://kind.sigs.k8s.io/) and [MetalLB](https://metallb.io) under the hood to provision Kubernetes clusters and assign local IPs to `LoadBalancer` `Services`. It has some [limitations](https://kind.sigs.k8s.io/docs/user/loadbalancer/) in Mac and Windows which will make the operator unable to connect to MariaDB via the `LoadBalancer` `Service`, leading to errors when reconciling SQL-related resources. Alternatively, use the [devcontainer](#devcontainer) flavour.

## Getting started

After having decided which [flavour](#flavours) to use and install the required dependencies, you will be able to use the `Makefile` targets we provide. For convenience, every development action has an associated `Makefile` target. You can list all of them by running `make`:
```bash
make

Usage:
  make <target>

General
  help             Display this help.

...

Install
  install-crds     Install CRDs.
  uninstall-crds   Uninstall CRDs.
  install          Install CRDs and dependencies for local development.
  install-samples  Install sample configuration.
  serviceaccount   Create long-lived ServiceAccount token for development.

Dev
  certs            Generates development certificates.
  lint             Lint.
  build            Build binary.
  test             Run tests.
  cover            Run tests and generate coverage.
  release          Test release locally.

Operator
  run              Run a controller from your host. 

...
```

## Running modes

There are two running modes, which are mutually exclusive and should not be mixed:

| | Host mode (preferred) | In-cluster mode |
|---|---|---|
| Operator runs | On your host, via `go run` | In the KIND cluster, deployed with Helm |
| Cluster | `mdb`, one checkout at a time (lease) | One slot cluster per checkout, `slot-<name>` |
| Networking | `make net`: MetalLB + `/etc/hosts` | None, the operator uses the cluster DNS |
| Image build | Not needed | On each `make run-in-cluster` |
| Integration tests | `make test-int*` on your host | `make test-int*-in-cluster`, as a `Job` in the slot cluster |

#### Host mode

Host mode is the preferred mode, as it does not require building an image and gives you the fastest feedback loop. It is used by the rest of this guide:

```bash
make cluster
make install
make net
make run
```

Only one checkout can use the host cluster at a time: host mode targets (`make net`, `make run`, `make test-int*`...) transparently acquire or renew a Kubernetes `Lease` (`default/mariadb-operator-dev`) for 4 hours. If another checkout holds it, they fail suggesting the in-cluster mode. Release it when you are done, so others can use the host cluster:

```bash
make host-lease-release
```

#### In-cluster mode

When the task calls for it, or the host cluster is leased by someone else, bind your checkout to a slot cluster, named after an issue or pull request:

```bash
make cluster-init SLOT=pr-1234
make cluster
make run-in-cluster
```

`make cluster-init` writes a git ignored `.cluster.mk` file, which makes all subsequent `make` invocations in the checkout target the slot cluster. Run `make mode` to print the mode and cluster of the checkout. Slot clusters are regular KIND contexts (`kind-slot-<name>`), so you can inspect them with `kubectl --context`, `kubectx` or `k9s`.

After changing the code, run `make run-in-cluster` again to rebuild the image and restart the operator. To run the integration tests, install their dependencies and stop the operator, as the tests run their own operator in-process. The tests run as a compiled test binary, so flags are passed with `TEST_ARGS` in its format, i.e. Ginkgo flags with the `--ginkgo.` prefix:

```bash
make install-in-cluster
make stop-in-cluster
make test-int-basic-in-cluster
make test-int-in-cluster TEST_ARGS="--ginkgo.focus=Galera"
make test-int-multi-cluster-in-cluster
```

When you are done, delete the slot cluster and unbind the checkout with `make delete-in-cluster`.

Running a target in the wrong mode fails fast with a `GUARD:` error. This is a safeguard, not a bug: switch to the right mode instead.

## Cluster

To start with, you will need a Kubernetes cluster for developing locally. You can provision a [KIND](https://kind.sigs.k8s.io/) cluster by using the following target:
```bash
make cluster
```
To decommission the cluster:
```bash
make cluster-delete
```

## Network

You can configure the network connectivity so the operator is able to resolve DNS and address MariaDB as if it was running in-cluster:
```bash
make net
```

This connectivity leverages [MetalLB](https://metallb.io) to assign local IPs to the `LoadBalancer` `Services` for the operator to connect to MariaDB. For this to happen, these local IPs need to be within the docker CIDR, which can be queried using:
```bash
make cidr
172.18.0.0/16
```

When deploying [example manifests](../examples/manifests/), take into account that `LoadBalancer` `Services` need to be within the docker CIDR to function properly, see:
- [examples/manifests/mariadb_v1alpha1_mariadb.yaml](https://github.com/mariadb-operator/mariadb-operator/blob/6f79a8e9e73977c433fb2d5c39a4b7210349b46c/examples/manifests/mariadb_v1alpha1_mariadb.yaml#L95)
- [examples/manifests/mariadb_v1alpha1_mariadb_replication.yaml](https://github.com/mariadb-operator/mariadb-operator/blob/160b7cc937c031f6faf7c1f50fcae78053faf766/examples/manifests/mariadb_v1alpha1_mariadb_replication.yaml#L87)
- [examples/manifests/mariadb_v1alpha1_mariadb_galera.yaml](https://github.com/mariadb-operator/mariadb-operator/blob/6f79a8e9e73977c433fb2d5c39a4b7210349b46c/examples/manifests/mariadb_v1alpha1_mariadb_galera.yaml#L102)


## Dependencies

You might need the following third party dependencies to test certain features of `mariadb-operator`, to install them run:

```bash
make install-prometheus
make install-cert-manager
make install-minio
make install-azurite
```

Some of this dependencies have ports mapped to the host (i.e. Grafana and Minio to expose the dashboard) so be sure to check the [forwarded ports](../.devcontainer/devcontainer.json) to access. This step requires running `make net` previously.

## Generate

This target generates code, CRDs and deployment manifests. Make sure to run this before pushing a commit, as it is required by the CI:

```bash
make gen
```

## Install

Install CRDs and everything you need to run the operator locally:

```bash
make install
```

## Build

Build the operator binary:

```bash
make build
```

Build the docker image and load it into KIND:

```bash
make docker-build
make docker-load
```

## Run

```bash
make cluster
make install
make net
make run
```

## Unit tests

```bash
make test
```

## Integration tests

```bash
make cluster
make install
make install-minio
make net
make test-int
make install-azurite
```