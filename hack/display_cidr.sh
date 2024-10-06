#!/bin/bash

set -euo pipefail

CIDR_PREFIX=$(go run ./hack/get_kind_cidr_prefix/main.go)
echo "${CIDR_PREFIX}.0.0/16"
