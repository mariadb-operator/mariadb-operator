#!/bin/bash

set -Eeox pipefail

# See: 
# https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/ci-pipeline.md
# https://github.com/redhat-marketplace/redhat-marketplace-operator/blob/9af66069a49f2eea5b03d170dd3f85e56c19ebed/hack/certify/certify.sh

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
SLEEP_LONG="${SLEEP_LONG:-5}"
SLEEP_SHORT="${SLEEP_SHORT:-2}"
CERT_NAMESPACE="${CERT_NAMESPACE:-certification}"
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

# Check Subscriptions: subscription-name, namespace
checksub () {
	echo "Waiting for Subscription $1 InstallPlan to complete."

	# Wait 2 resync periods for OLM to emit new installplan
	# sleep 60

	# Wait for the InstallPlan to be generated and available on status
	unset INSTALL_PLAN
	until oc get subscription $1 -n $2 --output=jsonpath={.status.installPlanRef.name}
	do
		sleep $SLEEP_SHORT
	done

	# Get the InstallPlan
	until [ -n "$INSTALL_PLAN" ]
	do
		sleep $SLEEP_SHORT
		INSTALL_PLAN=$(oc get subscription $1 -n $2 --output=jsonpath={.status.installPlanRef.name})
	done

	# Wait for the InstallPlan to Complete
	unset PHASE
	until [ "$PHASE" == "Complete" ]
	do
		PHASE=$(oc get installplan $INSTALL_PLAN -n $2 --output=jsonpath={.status.phase})
    if [ "$PHASE" == "Failed" ]; then
      set +x
      sleep 3
      echo "InstallPlan $INSTALL_PLAN for subscription $1 failed."
      echo "To investigate the reason of the InstallPlan failure run:"
      echo "oc describe installplan $INSTALL_PLAN -n $2"
      exit 1
    fi
		sleep $SLEEP_SHORT
	done
	
	# Get installed CluserServiceVersion
	unset CSV
	until [ -n "$CSV" ]
	do
		sleep $SLEEP_SHORT
		CSV=$(oc get subscription $1 -n $2 --output=jsonpath={.status.installedCSV})
	done
	
	# Wait for the CSV
	unset PHASE
	until [ "$PHASE" == "Succeeded" ]
	do
		PHASE=$(oc get clusterserviceversion $CSV -n $2 --output=jsonpath={.status.phase})
    if [ "$PHASE" == "Failed" ]; then
      set +x
      sleep 3
      echo "ClusterServiceVersion $CSV for subscription $1 failed."
      echo "To investigate the reason of the ClusterServiceVersion failure run:"
      echo "oc describe clusterserviceversion $CSV -n $2"
      exit 1
    fi
		sleep $SLEEP_SHORT
	done
}

# Verify Subscriptions
checksub openshift-pipelines-operator-rh openshift-operators

# Switch to certification namespace
oc delete ns $CERT_NAMESPACE --ignore-not-found
oc adm new-project $CERT_NAMESPACE
oc project $CERT_NAMESPACE

# Switch to certification namespace
oc delete ns $CERT_NAMESPACE --ignore-not-found
oc adm new-project $CERT_NAMESPACE
oc project $CERT_NAMESPACE

# Wait for the tekton serviceaccount to generate
echo "Waiting for ServiceAccount pipeline in namespace $CERT_NAMESPACE to generate."
until oc -n $CERT_NAMESPACE get serviceaccount pipeline
do
  sleep 5s
done

# Create the kubeconfig used by the certification pipeline
oc delete secret kubeconfig --ignore-not-found
oc create secret generic kubeconfig --from-file=kubeconfig=$KUBECONFIG

# Import redhat catalogs
oc import-image certified-operator-index:v4.16 \
  --request-timeout=5m \
  --from=registry.redhat.io/redhat/certified-operator-index:v4.16 \
  --reference-policy local \
  --scheduled \
  --confirm

# Install the Certification Pipeline
if [ ! -d "operator-pipelines" ]; then
  git clone https://github.com/redhat-openshift-ecosystem/operator-pipelines
fi
cd operator-pipelines
git checkout v1.0.122
cd -

# Create a new SCC
oc apply -f operator-pipelines/ansible/roles/operator-pipeline/templates/openshift/openshift-pipelines-custom-scc.yml
# Add SCC to a pipeline service account
oc adm policy add-scc-to-user pipelines-custom-scc -z pipeline

# Workaround some files that fail webhook validation
oc apply -R -f operator-pipelines/ansible/roles/operator-pipeline/templates/openshift/pipelines || true
oc apply -R -f operator-pipelines/ansible/roles/operator-pipeline/templates/openshift/tasks || true

# Run the Pipeline
tkn pipeline start operator-ci-pipeline \
  --param git_repo_url=$CERTIFIED_REPO \
  --param git_branch=$CERTIFIED_BRANCH \
  --param upstream_repo_name=redhat-openshift-ecosystem/certified-operators \
  --param bundle_path=$BUNDLE_PATH \
  --param env=prod \
  --workspace name=pipeline,volumeClaimTemplateFile=operator-pipelines/templates/workspace-template.yml \
  --pod-template operator-pipelines/templates/crc-pod-template.yml \
  --showlog \
  --use-param-defaults \
  --param submit=false