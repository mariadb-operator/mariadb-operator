#!/bin/bash

set -euo pipefail

# see: https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/ci-pipeline.md

if [ -z "$CERTIFIED_REPO" ]; then
  echo "CERTIFIED_REPO env variable must be provided"
  exit 1
fi
if [ -z "$CERTIFIED_BRANCH" ]; then
  echo "CERTIFIED_BRANCH env variable must be provided"
  exit 1
fi
if [ -z "$BUNDLE_PATH" ]; then
  echo "BUNDLE_PATH env variable must be provided"
  exit 1
fi
OPENSHIFT_VERSION="$(oc version -o json | jq -r .openshiftVersion)"

if ! kubectl get ns certification > /dev/null 2>&1
then
  oc adm new-project certification
fi
oc project certification

oc delete secret kubeconfig || true
oc create secret generic kubeconfig --from-file=kubeconfig="$HOME/.kube/config"

LATEST_OPENSHIFT_PIPELINES_OPERATOR_CSV="$(
oc get packagemanifests openshift-pipelines-operator-rh \
    --template '{{ range .status.channels }}{{ if eq .name "latest" }}{{ .currentCSV }}{{ "\n" }}{{ end }}{{ end }}')"

cat << EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-pipelines-operator-rh
  namespace: openshift-operators
spec:
  channel: latest
  installPlanApproval: Automatic
  name: openshift-pipelines-operator-rh
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  startingCSV: $LATEST_OPENSHIFT_PIPELINES_OPERATOR_CSV
EOF

oc import-image certified-operator-index:v"${OPENSHIFT_VERSION%.*}" \
  --from=registry.redhat.io/redhat/certified-operator-index:v"${OPENSHIFT_VERSION%.*}" \
  --request-timeout=5m \
  --reference-policy local \
  --scheduled \
  --confirm > /dev/null
oc tag registry.redhat.io/redhat/certified-operator-index:v"${OPENSHIFT_VERSION%.*}" certified-operator-index:v"${OPENSHIFT_VERSION%.*}"

oc import-image redhat-marketplace-index:v"${OPENSHIFT_VERSION%.*}" \
  --from=registry.redhat.io/redhat/redhat-marketplace-index:v"${OPENSHIFT_VERSION%.*}" \
  --request-timeout=5m \
  --reference-policy local \
  --scheduled \
  --confirm > /dev/null
oc tag registry.redhat.io/redhat/redhat-marketplace-index:v"${OPENSHIFT_VERSION%.*}" redhat-marketplace-index:v"${OPENSHIFT_VERSION%.*}"

oc adm policy add-scc-to-user anyuid -z pipeline

if [ ! -d "operator-pipelines" ]; then
  git clone https://github.com/redhat-openshift-ecosystem/operator-pipelines
fi
cd operator-pipelines
oc apply -R -f ansible/roles/operator-pipeline/templates/openshift/pipelines/operator-ci-pipeline.yml
oc apply -R -f ansible/roles/operator-pipeline/templates/openshift/tasks

oc apply -f ansible/roles/operator-pipeline/templates/openshift/openshift-pipelines-custom-scc.yml
oc adm policy add-scc-to-user pipelines-custom-scc -z pipeline

tkn pipeline start operator-ci-pipeline \
  --param git_repo_url=$CERTIFIED_REPO \
  --param git_branch=$CERTIFIED_BRANCH \
  --param bundle_path=$BUNDLE_PATH \
  --param env=prod \
  --workspace name=pipeline,volumeClaimTemplateFile=templates/workspace-template.yml \
  --showlog \
  --use-param-defaults \
  --param submit=false