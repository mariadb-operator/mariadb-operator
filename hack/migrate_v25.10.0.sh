#!/bin/bash

set -eo pipefail

MARIADB_INPUT="$1"
MARIADB_OUTPUT="migrated.$MARIADB_INPUT"
MARIADB_STATUS_OUTPUT="status.$MARIADB_INPUT"

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

HAS_WAIT_POINT=$("$YQ" ".spec.replication.replica.waitPoint != null" "$MARIADB_OUTPUT")
if [[ $HAS_WAIT_POINT ]]; then
  echo ".spec.replication.replica.waitPoint is present and not null, will migrate"
  "$YQ" '
    .spec.replication.waitPoint = .spec.replication.replica.waitPoint |
    del(.spec.replication.replica.waitPoint)
  ' -i "$MARIADB_OUTPUT"
fi

HAS_CONNECTION_TIMEOUT=$("$YQ" ".spec.replication.replica.connectionTimeout != null" "$MARIADB_OUTPUT")
if [[ $HAS_CONNECTION_TIMEOUT ]]; then
  echo ".spec.replication.replica.connectionTimeout is present and not null, will migrate"
  "$YQ" '
    .spec.replication.ackTimeout = .spec.replication.replica.connectionTimeout |
    del(.spec.replication.replica.connectionTimeout)
  ' -i "$MARIADB_OUTPUT"
fi

 echo "Creating status patch..."
 cp "$MARIADB_INPUT" "$MARIADB_STATUS_OUTPUT"

"$YQ" '. |= pick(["status"])' -i "$MARIADB_STATUS_OUTPUT"

# Show a summary if `diff` is installed.
if command_exists diff; then
  echo "Here are all the differences"
  # 
  diff -ubB $MARIADB_INPUT $MARIADB_OUTPUT
fi