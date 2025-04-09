import abc
import logging
from typing import Dict, Any

import pykube
import pytest
from pykube import HTTPClient
from pykube.objects import NamespacedAPIObject
from pytest_helm_charts.clusters import Cluster
from pytest_helm_charts.utils import wait_for_objects_condition

logger = logging.getLogger(__name__)

class NamespacedKonfigureOperatorCR(NamespacedAPIObject, abc.ABC):
    pass

class ManagementClusterConfigurationCR(NamespacedKonfigureOperatorCR):
    version = "konfigure.giantswarm.io/v1alpha1"
    endpoint = "managementclusterconfigurations"
    kind = "ManagementClusterConfiguration"

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
def test_mcc_working(kube_cluster: Cluster) -> None:
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

def mcc_cr_ready(cr: NamespacedKonfigureOperatorCR) -> bool:
    conditions = cr.obj.get("status", {}).get("conditions", [])

    for condition in conditions:
        if condition.get("type", "") == "Ready" and condition.get("status", "") == "True":
            return True

    return False
