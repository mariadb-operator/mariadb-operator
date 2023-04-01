#!/bin/bash

set -euo pipefail

IP=$1
HOSTNAME=$2

if grep -q "^$IP\s*$HOSTNAME" /etc/hosts; then
  echo "\"$HOSTNAME\" host already exists in /etc/hosts"
else
  echo "Adding \"$HOSTNAME\" to /etc/hosts";
  sudo -- sh -c -e "echo '# mariadb-operator\n$IP\t$HOSTNAME' >> /etc/hosts";
fi