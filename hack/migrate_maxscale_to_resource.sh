#!/bin/bash

##########################################################################################
## Migration script that removes the `maxScale` field in the given MariaDB CR
## We are relying that the operator already has created the MaxScale Resource.
##########################################################################################

set -eo pipefail

MARIADB_INPUT="$1"
MARIADB_OUTPUT="migrated.$MARIADB_INPUT"

if [ -z "$MARIADB_INPUT" ]; then
  echo "Error: MariaDB manifest file must be provided as the first argument."
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

echo "Cleaning up deprecated fields"
"$YQ" '
  del(.spec.maxScale)
' -i "$MARIADB_OUTPUT"

# Show a summary if `diff` is installed.
if command_exists diff; then
  echo "Here are all the differences"
  # 
  diff -ubB $MARIADB_INPUT $MARIADB_OUTPUT
fi

echo "Your migrated manifest is at $MARIADB_OUTPUT"