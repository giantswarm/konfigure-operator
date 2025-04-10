import pytest
from pytest_helm_charts.clusters import Cluster


@pytest.fixture(scope='module')
def fixtures(kube_cluster: Cluster):
    print("This should be invoked once per module!")
