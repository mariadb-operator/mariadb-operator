#!/bin/bash

CONFIG="$( dirname "${BASH_SOURCE[0]}" )"/config
if [ -z "$PROMETHEUS_VERSION" ]; then 
  PROMETHEUS_VERSION="33.2.0"
fi

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install \
  --version $PROMETHEUS_VERSION \
  -n kube-prometheus-stack --create-namespace \
  -f $CONFIG/kube-prometheus-stack.yaml \
  kube-prometheus-stack prometheus-community/kube-prometheus-stack