import abc
import base64
import logging
import os
from pathlib import Path
from typing import Dict, Any

import pykube
import pytest
import yaml
from pykube.objects import NamespacedAPIObject
from pytest_helm_charts.clusters import Cluster, ExistingCluster
from pytest_helm_charts.flux.git_repository import make_git_repository_obj, wait_for_git_repositories_to_be_ready
from pytest_helm_charts.giantswarm_app_platform.app import make_app_object
from pytest_helm_charts.giantswarm_app_platform.catalog import make_catalog_obj
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.utils import wait_for_objects_condition
from yaml import SafeLoader

logger = logging.getLogger(__name__)

# The operator only reconciles Konfigurations in its privileged namespaces (giantswarm and
# flux-giantswarm by default). The GitRepository and schema stay in default on purpose.
KONFIGURATION_NAMESPACE = "giantswarm"


def ensure_namespace(kube_client: pykube.HTTPClient, name: str) -> None:
    namespace = pykube.Namespace(kube_client, {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": name}})
    if not namespace.exists():
        namespace.create()


class NamespacedKonfigureOperatorCR(NamespacedAPIObject, abc.ABC):
    pass


class KonfigurationSchemaCR(NamespacedKonfigureOperatorCR):
    version = "konfigure.giantswarm.io/v1alpha1"
    endpoint = "konfigurationschemas"
    kind = "KonfigurationSchema"


class KonfigurationCR(NamespacedKonfigureOperatorCR):
    version = "konfigure.giantswarm.io/v1alpha1"
    endpoint = "konfigurations"
    kind = "Konfiguration"


@pytest.fixture(scope="module")
def setup(kube_cluster: Cluster):
    ensure_namespace(kube_cluster.kube_client, KONFIGURATION_NAMESPACE)

    # giantswarm catalog
    giantswarm_catalog = make_catalog_obj(kube_cluster.kube_client, "giantswarm", "default", "https://giantswarm.github.io/giantswarm-catalog/")
    if giantswarm_catalog.exists():
        giantswarm_catalog.update()
    else:
        giantswarm_catalog.create()

    # flux-app
    configured_flux_app = make_app_object(
        kube_cluster.kube_client,
        app_name="flux-app",
        app_version="1.4.3",
        catalog_name="giantswarm",
        catalog_namespace="default",
        namespace="default",
        deployment_namespace="default",
        config_values={
            "global": {
                "podSecurityStandards": {
                    "enforced": True
                },
            },
            "verticalPodAutoscaler": {
                "enabled": False
            }
        },
    )

    if configured_flux_app.app_cm.exists():
        configured_flux_app.app_cm.update()
    else:
        configured_flux_app.app_cm.create()

    if configured_flux_app.app.exists():
        configured_flux_app.app.update()
    else:
        configured_flux_app.app.create()

    wait_for_deployments_to_run(
        kube_cluster.kube_client,
        deployment_names=["source-controller"],
        deployments_namespace="default",
        timeout_sec=60,
    )

    # SOPS AGE keys
    secret_example_config_sops_age_key = {
        "apiVersion": "v1",
        "kind": "Secret",
        "metadata": {
            "name": "sops-keys",
            "namespace": "default",
            "labels": {
                "konfigure.giantswarm.io/data": "sops-keys"
            },
        },
        "data": {
            "example-configs.agekey": "QUdFLVNFQ1JFVC1LRVktMU5aSERFWUFDWVZDTTZTNVhXSlkwRkxKTFdNVkhRTkRDWldHN1Q1VzZMV1o0NFM4VENBOVM2OTg0VjIK"
        },
        "type": "Opaque",
    }

    secret_obj_example_config_sops_age_key = pykube.Secret(kube_cluster.kube_client, secret_example_config_sops_age_key)

    if secret_obj_example_config_sops_age_key.exists():
        secret_obj_example_config_sops_age_key.update()
    else:
        secret_obj_example_config_sops_age_key.create()

    # example-configs gitrepo
    gitrepo_example_configs = make_git_repository_obj(
        kube_cluster.kube_client,
        name="example-configs",
        namespace="default",
        interval="30s",
        repo_url="https://github.com/giantswarm/example-configs",
        repo_branch="main",
    )

    if gitrepo_example_configs.exists():
        gitrepo_example_configs.update()
    else:
        gitrepo_example_configs.create()

    wait_for_git_repositories_to_be_ready(
        kube_cluster.kube_client,
        git_repo_names=["example-configs"],
        git_repo_namespace="default",
        timeout_sec=45,
    )

    # CRDs
    install_crd(kube_cluster, "../../config/crd/bases/konfigure.giantswarm.io_konfigurations.yaml")
    install_crd(kube_cluster, "../../config/crd/bases/konfigure.giantswarm.io_konfigurationschemas.yaml")


@pytest.mark.functional
def test_api_working(kube_cluster: Cluster) -> None:
    """Very minimalistic example of using the [kube_cluster](pytest_helm_charts.fixtures.kube_cluster)
    fixture to get an instance of [Cluster](pytest_helm_charts.clusters.Cluster) under test
    and access its [kube_client](pytest_helm_charts.clusters.Cluster.kube_client) property
    to get access to Kubernetes API of cluster under test.
    Please refer to [pykube](https://pykube.readthedocs.io/en/latest/api/pykube.html) to get docs
    for [HTTPClient](https://pykube.readthedocs.io/en/latest/api/pykube.html#pykube.http.HTTPClient).
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1


raw_schema = """---
variables:
  - name: installation
    required: true
  - name: app
    required: true
layers:
  - id: defaults
    path:
      directory: default
      required: true
    values:
      path:
        directory: ""
      configMap:
        name: config.yaml
        required: true
    templates:
      path:
        directory: apps/<< app >>
        required: true
      configMap:
        name: configmap-values.yaml.template
        required: true
        values:
          merge:
            strategy: ConfigMapsInLayerOrder
      secret:
        name: secret-values.yaml.template
        required: false
        values:
          merge:
            strategy: ConfigMapsAndSecretsInLayerOrder
  - id: management-cluster
    path:
      directory: installations/<< installation >>
      required: true
    values:
      path:
        directory: ""
      configMap:
        name: config.yaml.patch
        required: true
      secret:
        name: secret.yaml
        required: true
    templates:
      path:
        directory: apps/<< app >>
        required: false
      configMap:
        name: configmap-values.yaml.patch
        required: false
        values:
          merge:
            strategy: ConfigMapsInLayerOrder
      secret:
        name: secret-values.yaml.patch
        required: false
        values:
          merge:
            strategy: SameTypeFromCurrentLayer
includes:
  - id: include
    function:
      name: include
    path:
      directory: include
      required: true # unused
    extension: .yaml.template
  - id: include-self
    function:
      name: includeSelf
    path:
      directory: include-self
      required: false # unused
    extension: .yaml.template
"""

@pytest.mark.functional
def test_konfiguration_working(setup, kube_cluster: Cluster) -> None:
    # THIS IS A MUST TO BE ABLE TO INTERACT WITH CRDs APPLIED DURING TEST COS THE ORIGINAL CLIENT ALREADY DID DISCOVERY
    # AND IS BEING REUSED ACROSS TESTS
    fresh_kube_client = ExistingCluster(kube_config_path=kube_cluster.kube_config_path).create()

    konfiguration_schema_cr: Dict[str, Any] = {
        "apiVersion": KonfigurationSchemaCR.version,
        "kind": KonfigurationSchemaCR.kind,
        "metadata": {
            "name": "example-schema",
            "namespace": "default",
        },
        "spec": {
            "raw": {
                "content": raw_schema,
            },
        }
    }

    example_schema = KonfigurationSchemaCR(fresh_kube_client, konfiguration_schema_cr)

    if example_schema.exists():
        example_schema.update()
    else:
        example_schema.create()

    konfiguration_cr = make_konfiguration("example-1", KONFIGURATION_NAMESPACE)

    example_konfiguration = KonfigurationCR(fresh_kube_client, konfiguration_cr)

    if example_konfiguration.exists():
        example_konfiguration.update()
    else:
        example_konfiguration.create()

    wait_for_objects_condition(
        fresh_kube_client,
        KonfigurationCR,
        ["example-1"],
        KONFIGURATION_NAMESPACE,
        cr_ready,
        120,
        False,
    )

    cm_app_1_ex_1 = pykube.ConfigMap.objects(fresh_kube_client).get(name="app-1-kfg-ex1", namespace="default")

    values_app_1_ex_1 = yaml.load(cm_app_1_ex_1.obj.get("data", {}).get("configmap-values.yaml", ""), SafeLoader)

    assert values_app_1_ex_1.get("foo", "") == "override"
    assert values_app_1_ex_1.get("bar", "") == "world"
    assert values_app_1_ex_1.get("new", "") == "value"

    assert values_app_1_ex_1.get("todo", {}).get("items", []) == ["item-1", "item-2", "item-3"]

    secret_app_1_ex_1 = pykube.Secret.objects(fresh_kube_client).get(name="app-1-kfg-ex1", namespace="default")

    secret_values_app_1_ex_1 = yaml.load(base64.b64decode(secret_app_1_ex_1.obj.get("data", {}).get("secret-values.yaml", "")), SafeLoader)

    assert secret_values_app_1_ex_1.get("example") == "example"

@pytest.mark.functional
def test_konfiguration_refused_outside_privileged_namespaces(setup, kube_cluster: Cluster) -> None:
    fresh_kube_client = ExistingCluster(kube_config_path=kube_cluster.kube_config_path).create()

    konfiguration = KonfigurationCR(fresh_kube_client, make_konfiguration("example-unprivileged", "default"))

    if konfiguration.exists():
        konfiguration.update()
    else:
        konfiguration.create()

    wait_for_objects_condition(
        fresh_kube_client,
        KonfigurationCR,
        ["example-unprivileged"],
        "default",
        cr_refused_namespace_not_allowed,
        60,
        False,
    )


def make_konfiguration(name: str, namespace: str) -> Dict[str, Any]:
    return {
        "apiVersion": KonfigurationCR.version,
        "kind": KonfigurationCR.kind,
        "metadata": {
            "name": name,
            "namespace": namespace,
        },
        "spec": {
            "targets": {
                "schema": {
                    "reference": {
                        "name": "example-schema",
                        "namespace": "default"
                    }
                },
                "defaults": {
                    "variables": [
                        {
                            "name": "installation",
                            "value": "installation-1"
                        }
                    ]
                },
                "iterations": {
                    "app-1": {
                        "variables": [
                            {
                                "name": "app",
                                "value": "app-1"
                            }
                        ]
                    }
                }
            },
            "destination": {
                "naming": {
                    "suffix": "kfg-ex1"
                },
                "namespace": "default"
            },
            "reconciliation": {
                "retryInterval": "10s",
                "interval": "1m"
            },
            "sources": {
                "flux": {
                    "gitRepository": {
                        "namespace": "default",
                        "name": "example-configs"
                    }
                }
            }
        }
    }


def install_crd(kube_cluster, relative_path_to_crd_file: str) -> None:
    dir_path = os.path.dirname(os.path.realpath(__file__))

    text = Path(f"{dir_path}/{relative_path_to_crd_file}").read_text()
    dictionary = yaml.load(text, SafeLoader)

    crd = pykube.CustomResourceDefinition(kube_cluster.kube_client, dictionary)

    if crd.exists():
        crd.update()
    else:
        crd.create()


def cr_refused_namespace_not_allowed(cr: NamespacedKonfigureOperatorCR) -> bool:
    conditions = cr.obj.get("status", {}).get("conditions", [])

    for condition in conditions:
        if (
            condition.get("type", "") == "Ready"
            and condition.get("status", "") == "False"
            and condition.get("reason", "") == "NamespaceNotAllowed"
        ):
            return True

    return False


def cr_ready(cr: NamespacedKonfigureOperatorCR) -> bool:
    conditions = cr.obj.get("status", {}).get("conditions", [])

    for condition in conditions:
        if condition.get("type", "") == "Ready" and condition.get("status", "") == "True":
            return True

    return False
