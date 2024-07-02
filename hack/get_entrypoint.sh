#!/bin/bash

set -euo pipefail

if [ -z "$MARIADB_DOCKER_REPO" ]; then 
  echo "MARIADB_DOCKER_REPO environment variable is mandatory"
  exit 1
fi
if [ -z "$MARIADB_DOCKER_COMMIT_HASH" ]; then 
  echo "MARIADB_DOCKER_COMMIT_HASH environment variable is mandatory"
  exit 1
fi
if [ -z "$MARIADB_ENTRYPOINT_PATH" ]; then 
  echo "MARIADB_ENTRYPOINT_PATH environment variable is mandatory"
  exit 1
fi

# Clone the repository
if [ -d "mariadb-docker" ]; then
  rm -rf mariadb-docker
fi
echo "Cloning repository $MARIADB_DOCKER_REPO"
git clone -q --no-progress "$MARIADB_DOCKER_REPO" mariadb-docker
cd mariadb-docker
echo "Checking out commit $MARIADB_DOCKER_COMMIT_HASH"
git checkout -q --no-progress "$MARIADB_DOCKER_COMMIT_HASH"
cd -

# Prepare the entrypoint directory
if [ -d "$MARIADB_ENTRYPOINT_PATH" ]; then
  rm -rf "$MARIADB_ENTRYPOINT_PATH"
fi

for VERSION_DIR in $(find mariadb-docker/ -maxdepth 1 -type d -regex 'mariadb-docker/[0-9]+\.[0-9]+$' | sort -V); do
  VERSION=$(basename "$VERSION_DIR")
  ENTRYPOINT="$VERSION_DIR/docker-entrypoint.sh"
  
  if [ -f "$ENTRYPOINT" ]; then
    DEST="$MARIADB_ENTRYPOINT_PATH/$VERSION"
    mkdir -p "$DEST"
    cp "$ENTRYPOINT" "$DEST"
    echo "Copied docker-entrypoint.sh for version \"$VERSION\""
  else
    echo "❌ Error: docker-entrypoint.sh not found for version \"$VERSION\""
    exit 1
  fi
done

# Cleanup
rm -rf mariadb-docker
echo "Done!"