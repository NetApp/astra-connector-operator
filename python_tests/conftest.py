""" Outermost conftest file. Used for common fixtures. All inner contest/pytest suites inherit this file. """
import pytest
from test_utils.k8s_helper import K8sHelper
from test_utils import app_vault, buckets, astra_connector
import uuid
from pytest.management_tests import config
from collections import namedtuple

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
def k8s_helper(kubeconfig):
    return K8sHelper(kubeconfig)


# --------------------------
# cr_helper Fixture and Class
# --------------------------
class CrHelper:

    def __init__(self, k8s_helper):
        self.app_vault = app_vault.AppVaultHelper(k8s_helper)
        self.astra_connector = astra_connector.AstraConnectorHelper(k8s_helper)


@pytest.fixture(scope="session")
def cr_helper(k8s_helper):
    return CrHelper(k8s_helper)

# -------------------------------
# End cr_helper Fixture and Class
# -------------------------------


@pytest.fixture(scope="session")
def shared_bucket(s3_creds) -> buckets.Bucket:
    bucket_name = f"test-bucket-{str(uuid.uuid4())[:8]}"
    return buckets.Bucket(bucket_name, s3_creds.host, s3_creds.access_key, s3_creds.secret_key)


@pytest.fixture(scope="session")
def shared_app_vault(cr_helper, shared_bucket):
    secret_name = f"app-vault-secret-{str(uuid.uuid4())[:8]}"
    cr_helper.app_vault.create_app_vault_secret(
        namespace=config.DEFAULT_CONNECTOR_NAMESPACE,
        secret_name=secret_name,
        access_key=shared_bucket.access_key,
        secret_key=shared_bucket.secret_key
    )
    app_vault_name = f"test-app-vault-{str(uuid.uuid4())[:8]}"
    return cr_helper.app_vault.create_app_vault(
        name=app_vault_name,
        namespace=config.DEFAULT_CONNECTOR_NAMESPACE,
        bucket_name=shared_bucket.bucket_name,
        bucket_host=shared_bucket.host,
        secret_name=secret_name,
        provider_type="generic-s3")

