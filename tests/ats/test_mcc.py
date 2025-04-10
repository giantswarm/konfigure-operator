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
from pytest_helm_charts.clusters import Cluster
from pytest_helm_charts.flux.git_repository import make_git_repository_obj, wait_for_git_repositories_to_be_ready
from pytest_helm_charts.giantswarm_app_platform.app import make_app_object, ConfiguredApp
from pytest_helm_charts.giantswarm_app_platform.catalog import make_catalog_obj
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.utils import wait_for_objects_condition
from yaml import SafeLoader

logger = logging.getLogger(__name__)

class NamespacedKonfigureOperatorCR(NamespacedAPIObject, abc.ABC):
    pass

class ManagementClusterConfigurationCR(NamespacedKonfigureOperatorCR):
    version = "konfigure.giantswarm.io/v1alpha1"
    endpoint = "managementclusterconfigurations"
    kind = "ManagementClusterConfiguration"

@pytest.fixture(scope="module")
def setup(kube_cluster: Cluster):
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
    dir_path = os.path.dirname(os.path.realpath(__file__))

    text_mcc = Path(f"{dir_path}/../../config/crd/bases/konfigure.giantswarm.io_managementclusterconfigurations.yaml").read_text()
    dict_mcc = yaml.load(text_mcc, SafeLoader)

    crd_mcc = pykube.CustomResourceDefinition(kube_cluster.kube_client, dict_mcc)

    if crd_mcc.exists():
        crd_mcc.update()
    else:
        crd_mcc.create()


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

@pytest.mark.functional
def test_mcc_working(setup, kube_cluster: Cluster) -> None:
    cr: Dict[str, Any] = {
        "apiVersion": ManagementClusterConfigurationCR.version,
        "kind": ManagementClusterConfigurationCR.kind,
        "metadata": {
            "name": "example-1",
            "namespace": "default",
        },
        "spec": {
            "configuration": {
                "applications": {
                    "excludes": {
                        "regexMatchers": [],
                        "exactMatchers": []
                    },
                    "includes": {
                        "regexMatchers": [],
                        "exactMatchers": [
                            "app-1"
                        ]
                    }
                },
                "cluster": {
                    "name": "installation-1"
                }
            },
            "destination": {
                "naming": {
                    "suffix": "ex1"
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
                    },
                    "service": {
                        "url": "source-controller.default.svc"
                    }
                }
            },
        }
    }

    example1 = ManagementClusterConfigurationCR(kube_cluster.kube_client, cr)

    if example1.exists():
        example1.update()
    else:
        example1.create()

    wait_for_objects_condition(
        kube_cluster.kube_client,
        ManagementClusterConfigurationCR,
        ["example-1"],
        "default",
        mcc_cr_ready,
        120,
        False,
    )

    cm_app_1_ex_1 = pykube.ConfigMap.objects(kube_cluster.kube_client).get(name="app-1-ex1", namespace="default")

    values_app_1_ex_1 = yaml.load(cm_app_1_ex_1.obj.get("data", {}).get("configmap-values.yaml", ""), SafeLoader)

    assert values_app_1_ex_1.get("foo", "") == "override"
    assert values_app_1_ex_1.get("bar", "") == "world"
    assert values_app_1_ex_1.get("new", "") == "value"

    assert values_app_1_ex_1.get("todo", {}).get("items", []) == ["item-1", "item-2", "item-3"]

    secret_app_1_ex_1 = pykube.Secret.objects(kube_cluster.kube_client).get(name="app-1-ex1", namespace="default")

    secret_values_app_1_ex_1 = yaml.load(base64.b64decode(secret_app_1_ex_1.obj.get("data", {}).get("secret-values.yaml", "")), SafeLoader)

    assert secret_values_app_1_ex_1.get("example") == "example"


def mcc_cr_ready(cr: NamespacedKonfigureOperatorCR) -> bool:
    conditions = cr.obj.get("status", {}).get("conditions", [])

    for condition in conditions:
        if condition.get("type", "") == "Ready" and condition.get("status", "") == "True":
            return True

    return False
