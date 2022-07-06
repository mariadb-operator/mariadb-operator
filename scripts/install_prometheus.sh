#!/bin/bash

CONFIG="$( dirname "${BASH_SOURCE[0]}" )"/config

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install \
  -n kube-prometheus-stack --create-namespace \
  -f $CONFIG/kube-prometheus-stack.yaml \
  kube-prometheus-stack prometheus-community/kube-prometheus-stack