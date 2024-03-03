#!/bin/bash

set -eo pipefail

MARIADB_INPUT="$1"
MARIADB_OUTPUT="migrated.$MARIADB_INPUT"
MARIADB_STATUS_OUTPUT="status.$MARIADB_INPUT"

if [ -z "$MARIADB_INPUT" ]; then
  echo "Error: MariaDB manifest file from a version older than v0.0.26 must be provided as the first argument."
  echo "Usage: $0 mariadb.yaml"
  exit 1
fi

YQ=/tmp/yq
function setup_yq() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case $ARCH in
    x86_64)
      ARCH="amd64"
      ;;
    aarch64|arm64|armv8)
      ARCH="arm64"
      ;;
    i386)
      ARCH="386"
      ;;
    *)
      echo "Unsupported architecture: $ARCH" >&2
      exit 1
      ;;
  esac

  YQ_VERSION="4.42.1"
  YQ_BINARY="yq_${OS}_${ARCH}"
  YQ_DOWNLOAD_URL="https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/${YQ_BINARY}"

  echo "Downloading yq version ${YQ_VERSION}..."
  if ! wget -q "$YQ_DOWNLOAD_URL" -O "$YQ"; then
    echo "Failed to download yq binary." >&2
    exit 1
  fi

  chmod +x "$YQ"
}

if command -v yq &> /dev/null; then
  YQ=$(command -v yq)
  echo "yq is already installed. Using the current installation: $YQ"
else
  echo "Setting up yq..."
  setup_yq
  echo "Installed yq: $YQ"
fi

"$YQ" --version

echo "Migrating MariaDB fields..."
cp "$MARIADB_INPUT" "$MARIADB_OUTPUT"

"$YQ" eval '
  .apiVersion = "k8s.mariadb.com/v1alpha1" |
  .spec.serviceAccountName = .metadata.name |
  .spec.storage.size = .spec.volumeClaimTemplate.resources.requests.storage |
  .spec.storage.volumeClaimTemplate = .spec.volumeClaimTemplate |
  del(.spec.volumeClaimTemplate) |
  del(.spec.ephemeralStorage)
' -i "$MARIADB_OUTPUT"

GALERA_ENABLED=$("$YQ" eval ".spec.galera != null and .spec.galera.enabled == true" "$MARIADB_INPUT")
if [ "$GALERA_ENABLED" = "true" ]; then
  echo "Migrating Galera fields..."
  "$YQ" eval '
    .spec.galera.config.volumeClaimTemplate= .spec.galera.volumeClaimTemplate |
    .spec.galera.config.reuseStorageVolume= false |
    .spec.galera.initContainer.image = "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26" |
    .spec.galera.agent.image = "ghcr.io/mariadb-operator/mariadb-operator:v0.0.26" |
    .spec.galera.recovery.clusterHealthyTimeout = "30s" |
    .spec.galera.recovery.clusterBootstrapTimeout = "10m" |
    .spec.galera.recovery.podRecoveryTimeout = "3m" |
    .spec.galera.recovery.podSyncTimeout = "3m" |
    del(.spec.galera.volumeClaimTemplate)
  ' -i "$MARIADB_OUTPUT"
fi

echo "Creating status patch..."
cp "$MARIADB_INPUT" "$MARIADB_STATUS_OUTPUT"

"$YQ" eval '. |= pick(["status"])' -i "$MARIADB_STATUS_OUTPUT"
