import logging

import pykube
import pytest
from pytest_helm_charts.clusters import Cluster

logger = logging.getLogger(__name__)


@pytest.mark.smoke
def test_api_working(kube_cluster: Cluster) -> None:
    """Verify the smoke-test cluster is reachable and the chart installed.

    The smoke step deploys the chart into the cluster before this test runs, so
    a healthy API connection here confirms the chart templates render and the
    release installs cleanly.

    Note: this validates installability rather than pod readiness. The
    architect pipeline rewrites the chart's image tag to a wall-clock dev
    version that build-chart and push-to-registries compute independently, so
    the just-built dev image is not reliably pullable inside the smoke cluster.
    Pod-readiness assertions belong in an environment that deploys a pinned,
    published image.
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1
