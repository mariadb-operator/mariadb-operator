#!/bin/bash

set -euo pipefail

if ! kubectl get ns certification > /dev/null 2>&1
then
  oc adm new-project certification
fi
oc project certification

oc delete secret kubeconfig || true
oc create secret generic kubeconfig --from-file=kubeconfig="$HOME/.kube/config"

oc adm policy add-scc-to-user anyuid -z pipeline

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