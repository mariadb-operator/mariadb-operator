#!/bin/bash

FOLDER="$1"
IMAGE="$2"
VERSION="$3"

if [ -z "$FOLDER" ] || [ -z "$IMAGE" ] || [ -z "$VERSION" ]; then
  echo "Usage: $0 path/examples ghcr.io/mariadb-operator/mariadb-operator v0.0.27"
  exit 1
fi

echo "Updating examples in folder '$FOLDER' to '$IMAGE:$VERSION'"

for file in "$FOLDER"/*.yaml; do
  if [ -f "$file" ] && [ -r "$file" ]; then
    sed -i "s|$IMAGE:[^ ]*|$IMAGE:$VERSION|g" "$file"
  else
    echo "Error: $file does not exist or is not readable."
  fi
done
