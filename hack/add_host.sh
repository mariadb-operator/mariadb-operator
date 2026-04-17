#!/bin/bash

set -euo pipefail

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <3 octet> <4 octet> <hostname>"
    exit 1
fi

CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix/main.go)
IP="${CIDR_PREFIX}.$1.$2"
HOSTNAME=$3

if grep -q "^$IP\s*$HOSTNAME" /etc/hosts; then
  echo "\"$HOSTNAME\" host already exists in /etc/hosts"
else
  echo "Adding \"$HOSTNAME\" to /etc/hosts";
  sudo -- sh -c -e "printf '# mariadb-operator\n%s\t%s\n' '$IP' '$HOSTNAME' >> /etc/hosts";
fi