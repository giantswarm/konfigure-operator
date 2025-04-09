#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

KUBE_CONFIG=$(kind get kubeconfig) kubectl apply -f "${SCRIPT_DIR}/../../config/crd/bases/konfigure.giantswarm.io_managementclusterconfigurations.yaml"

KUBE_CONFIG=$(kind get kubeconfig) kubectl apply -f - <<EOF
apiVersion: application.giantswarm.io/v1alpha1
kind: Catalog
metadata:
  name: giantswarm
  namespace: default
spec:
  description: 'This catalog holds Apps managed by Giant Swarm. '
  logoURL: /images/repo_icons/managed.png
  repositories:
  - URL: https://giantswarm.github.io/giantswarm-catalog/
    type: helm
  - URL: oci://giantswarmpublic.azurecr.io/giantswarm-catalog/
    type: oci
  storage:
    URL: https://giantswarm.github.io/giantswarm-catalog/
    type: helm
  title: Giant Swarm Catalog
EOF

KUBE_CONFIG=$(kind get kubeconfig) kubectl apply -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: flux-app-values
  namespace: default
data:
  values.yaml: |
    global:
      podSecurityStandards:
        enforced: true
    verticalPodAutoscaler:
      enabled: false
EOF

KUBE_CONFIG=$(kind get kubeconfig) kubectl apply -f - <<EOF
---
apiVersion: application.giantswarm.io/v1alpha1
kind: App
metadata:
  labels:
    app-operator.giantswarm.io/version: 0.0.0
  name: flux-app
  namespace: default
spec:
  catalog: giantswarm
  kubeConfig:
    inCluster: true
  userConfig:
    configMap:
      name: flux-app-values
      namespace: default
  name: flux-app
  namespace: default
  version: 1.4.3
EOF

echo "Waiting 10s for flux installation to be ready"
sleep 10

KUBE_CONFIG=$(kind get kubeconfig) kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  labels:
    konfigure.giantswarm.io/data: sops-keys
  name: sops-keys
  namespace: default
data:
  example-configs.agekey: QUdFLVNFQ1JFVC1LRVktMU5aSERFWUFDWVZDTTZTNVhXSlkwRkxKTFdNVkhRTkRDWldHN1Q1VzZMV1o0NFM4VENBOVM2OTg0VjIK
EOF

KUBE_CONFIG=$(kind get kubeconfig) kubectl apply -f - <<EOF
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: example-configs
  namespace: default
spec:
  interval: 30s
  ref:
    branch: main
  timeout: 1m
  url: https://github.com/giantswarm/example-configs
EOF

echo "Waiting 5s for example-configs to be ready"
sleep 5
