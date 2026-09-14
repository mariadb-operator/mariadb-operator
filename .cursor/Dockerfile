# Cloud Agent development image for mariadb-operator.
#
# Provides the full local-development toolchain described in docs/development.md:
# Go, Docker (for KIND), make, a C toolchain and the mariadb client. KIND-managed
# tools (kind, kubectl, helm, kustomize, controller-gen, envtest, ginkgo) are
# fetched into ./bin by `make` during the install phase.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
        git \
        make \
        gcc \
        build-essential \
        fuse-overlayfs \
        mariadb-client \
        sudo \
    && rm -rf /var/lib/apt/lists/*

# Docker Engine + buildx from Docker's official apt repository. Docker is used to
# provision KIND clusters and to build/load the operator image for local development.
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
    && chmod a+r /etc/apt/keyrings/docker.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
        > /etc/apt/sources.list.d/docker.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain (go.mod requires go >= 1.26.0).
ARG GO_VERSION=1.26.7
ARG GO_SHA256=ffb5f8de10c62550dfddab66b36b57030721e0a44a3218e9e1181d7b59f121ca
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz \
    && echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum -c - \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

# The ubuntu user (uid 1000) ships with the base image; grant it passwordless
# sudo and docker access so the start script can manage the Docker daemon.
RUN groupadd -f docker \
    && usermod -aG sudo,docker ubuntu \
    && printf 'ubuntu ALL=(ALL) NOPASSWD:ALL\n' > /etc/sudoers.d/ubuntu \
    && chmod 0440 /etc/sudoers.d/ubuntu

ENV GOPATH=/home/ubuntu/go
ENV PATH=/usr/local/go/bin:/workspace/bin:/home/ubuntu/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

USER ubuntu
