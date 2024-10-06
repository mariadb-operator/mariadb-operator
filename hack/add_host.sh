#!/bin/bash

set -euo pipefail

CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix/main.go)
IP="${CIDR_PREFIX}.0.$1"
HOSTNAME=$2

if grep -q "^$IP\s*$HOSTNAME" /etc/hosts; then
  echo "\"$HOSTNAME\" host already exists in /etc/hosts"
else
  echo "Adding \"$HOSTNAME\" to /etc/hosts";
  sudo -- sh -c -e "printf '# mariadb-operator\n%s\t%s\n' '$IP' '$HOSTNAME' >> /etc/hosts";
fi
