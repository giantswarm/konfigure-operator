#!/usr/bin/env bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

(
  cd "$SCRIPT_DIR"/../.. || exit

  make build-linux
  cp konfigure-operator-linux konfigure-operator

  docker buildx build -t gsoci.azurecr.io/giantswarm/konfigure-operator:black .

  kind load docker-image gsoci.azurecr.io/giantswarm/konfigure-operator:black

  echo "Cleaning up konfigure-operator helm release in default namespace"
  helm uninstall -n default konfigure-operator > /dev/null || true

  echo "Installing new konfigure-operator helm release in default namespace"
  helm install -n default -n default --set "image.tag=black" konfigure-operator ./helm/konfigure-operator

  kubectl rollout restart -n default deployment konfigure-operator
)
