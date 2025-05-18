#!/bin/bash

set -eo pipefail

if [ -d "csi-driver-host-path" ]; then
  rm -rf csi-driver-host-path
fi

git clone -q --no-progress https://github.com/kubernetes-csi/csi-driver-host-path.git

./csi-driver-host-path/deploy/kubernetes-latest/deploy.sh

kubectl apply -f ./csi-driver-host-path/examples/csi-storageclass.yaml