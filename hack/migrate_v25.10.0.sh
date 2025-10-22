#!/bin/bash

set -eo pipefail

MARIADB_INPUT="$1"
MARIADB_OUTPUT="migrated.$MARIADB_INPUT"

if [ -z "$MARIADB_INPUT" ]; then
  echo "Error: MariaDB manifest file from a version older than 25.8.4 must be provided as the first argument."
  echo "Usage: $0 mariadb.yaml"
  exit 1
fi

command_exists() {
  command -v "$1" >/dev/null 2>&1
}


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

if command_exists yq; then
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

echo "Setting .spec.replication.semiSyncEnabled=true"
"$YQ" '
  .spec.replication.semiSyncEnabled = true
' -i "$MARIADB_OUTPUT"

echo "Setting .spec.replication.gtidStrictMode=true"
"$YQ" '
  .spec.replication.gtidStrictMode = true
' -i "$MARIADB_OUTPUT"

HAS_CONN_RETRY=$("$YQ" ".spec.replication.replica.connectionRetries != null" "$MARIADB_OUTPUT")
if [[ "$HAS_CONN_RETRY" == "true" ]]; then
  echo ".spec.replication.replica.connectionRetries is present and not null, will migrate"
  "$YQ" '
    .spec.replication.replica.connectionRetrySeconds = .spec.replication.replica.connectionRetries |
    del(.spec.replication.replica.connectionRetries)
  ' -i "$MARIADB_OUTPUT"
fi

HAS_CONNECTION_TIMEOUT=$("$YQ" ".spec.replication.replica.connectionTimeout != null" "$MARIADB_OUTPUT")
if [[ "$HAS_CONNECTION_TIMEOUT" == "true" ]]; then
  echo ".spec.replication.replica.connectionTimeout is present and not null, will migrate"
  "$YQ" '
    .spec.replication.semiSyncAckTimeout = .spec.replication.replica.connectionTimeout |
    del(.spec.replication.replica.connectionTimeout)
  ' -i "$MARIADB_OUTPUT"
fi

HAS_WAIT_POINT=$("$YQ" ".spec.replication.replica.waitPoint != null" "$MARIADB_OUTPUT")
if [[ "$HAS_WAIT_POINT" == "true" ]]; then
  echo ".spec.replication.replica.waitPoint is present and not null, will migrate"
  "$YQ" '
    .spec.replication.semiSyncWaitPoint = .spec.replication.replica.waitPoint |
    del(.spec.replication.replica.waitPoint)
  ' -i "$MARIADB_OUTPUT"
fi

echo "Cleaning up deprecated fields"
"$YQ" '
  del(.spec.replication.probesEnabled) |
  del(.status.replicationStatus)
' -i "$MARIADB_OUTPUT"

# Show a summary if `diff` is installed.
if command_exists diff; then
  echo "Here are all the differences"
  # 
  diff -ubB $MARIADB_INPUT $MARIADB_OUTPUT
fi