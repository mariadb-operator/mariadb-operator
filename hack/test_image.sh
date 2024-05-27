#!/bin/bash

set -eo pipefail

# usage: GHA_TOKEN="$GHA_TOKEN" MARIADB_IMAGE="mariadb:10.11.7" test_image.sh

if [ -z "$GHA_TOKEN" ]; then 
  echo "GHA_TOKEN environment variable is mandatory"
  exit 1
fi
if [ -z "$MARIADB_IMAGE" ]; then 
  echo "MARIADB_IMAGE environment variable is mandatory"
  exit 1
fi

curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GHA_TOKEN" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/mariadb-operator/mariadb-operator/actions/workflows/test-image.yml/dispatches \
  -d "{\"ref\":\"main\",\"inputs\":{\"mariadb_image\":\"$MARIADB_IMAGE\"}}"