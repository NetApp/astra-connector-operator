""" Outermost conftest file. Used for common fixtures. All inner contest/pytest suites inherit this file. """
import pytest
import uuid
from collections import namedtuple
from test_utils.cluster import Cluster
from test_utils.buckets import BucketManager, Bucket


# Add custom pytest args
def pytest_addoption(parser):
    parser.addoption(
        "--kubeconfig", action="store", default="~/.kube/config", help="Path to the kubeconfig file"
    )
    parser.addoption(
        "--s3_host", action="store", default="not_set", help="S3 Host/IP"
    )
    parser.addoption(
        "--s3_secret_key", action="store", default="not_set", help="S3 Secret Key"
    )
    parser.addoption(
        "--s3_access_key", action="store", default="not_set", help="S3 Access Key"
    )


# -----------
# Parse Args
# -----------

@pytest.fixture(scope="session")
def kubeconfig(request):
    return request.config.getoption("--kubeconfig")


@pytest.fixture(scope="session")
def s3_secret_key(request):
    return request.config.getoption("--s3_secret_key")


@pytest.fixture(scope="session")
def s3_access_key(request):
    return request.config.getoption("--s3_access_key")


@pytest.fixture(scope="session")
def s3_host(request):
    return request.config.getoption("--s3_host")


@pytest.fixture(scope="session")
def s3_creds(s3_host, s3_access_key, s3_secret_key) -> namedtuple:
    S3_Creds = namedtuple('S3Creds', 'host access_key secret_key')
    return S3_Creds(host=s3_host, access_key=s3_access_key, secret_key=s3_secret_key)


# ---------------
# End Arg Parse #
# ---------------
@pytest.fixture(scope="session")
def bucket_manager(s3_creds):
    bucket_manager = BucketManager(s3_creds.host, s3_creds.access_key, s3_creds.secret_key)
    yield bucket_manager
    bucket_manager.cleanup_buckets()


@pytest.fixture(scope="session")
def app_clusters(kubeconfig, bucket_manager) -> list[Cluster]:
    # Note: only has one kubeconfig and appCluster now but is designed for N appClusters
    test_bucket = bucket_manager.create_bucket(f"test-bucket-{uuid.uuid4()[:8]}")
    app_clusters: list[Cluster] = [
        Cluster(kubeconfig, test_bucket)
    ]
    yield app_clusters

    # Cluster cleanup, runs after all tests are complete
    for cluster in app_clusters:
        cluster.cleanup()
