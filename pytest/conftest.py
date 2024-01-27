""" Outermost conftest file. Used for common fixtures. All inner contest/pytest suites inherit this file. """
import pytest
from test_utils import buckets, k8s_helper, app_vault
import uuid
import config


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

# ---------------
# End Arg Parse #
# ---------------


@pytest.fixture(scope="session")
def k8s_helper(kubeconfig):
    return k8s_helper.K8sHelper(kubeconfig)


@pytest.fixture(scope="session")
def bucket_manager(s3_host, s3_access_key, s3_secret_key):
    return buckets.BucketManager(s3_host, s3_access_key, s3_secret_key)


@pytest.fixture(scope="session")
def app_vault_manager(k8s_helper, bucket_manager):
    return app_vault.AppVaultManager(k8s_helper, bucket_manager)


@pytest.fixture(scope="session")
def shared_bucket(bucket_manager) -> buckets.Bucket:
    bucket_name = f"test-bucket-{str(uuid.uuid4())[:8]}"
    return bucket_manager.create_bucket(bucket_name)


@pytest.fixture(scope="session")
def shared_app_vault(app_vault_manager):
    app_vault_manager.create_app_vault_secret()
    app_vault_manager.create_app_vault()
