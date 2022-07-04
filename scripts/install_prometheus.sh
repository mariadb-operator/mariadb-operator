#!/bin/bash

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install \
  -n kube-prometheus-stack --create-namespace \
  kube-prometheus-stack prometheus-community/kube-prometheus-stack