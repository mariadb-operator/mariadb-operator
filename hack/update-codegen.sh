#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./bin/code-generator 2>/dev/null || echo ../code-generator)}

source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/client" \
  --output-pkg "github.com/mariadb-operator/mariadb-operator/pkg/client" \
  --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  "${SCRIPT_ROOT}/api"
