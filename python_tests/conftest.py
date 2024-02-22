""" Outermost conftest file. Used for common fixtures. All inner contest/pytest suites inherit this file. """
import pytest
from collections import namedtuple

import python_tests.defaults as defaults
import python_tests.test_utils.random as random
from python_tests.log import logger
from python_tests.test_utils.app_installer import App
from test_utils.buckets import BucketManager
from test_utils.cluster import Cluster


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
    parser.addoption(
        "--static_app", action="store_true", default=False,
        help="Set to true to test with a static app created outside of pytest"
    )
    parser.addoption(
        "--src_sc", action="store", default=defaults.SRC_STORAGE_CLASS, help="AppMirror source SC name"
    )
    parser.addoption(
        "--dst_sc", action="store", default=defaults.DST_STORAGE_CLASS, help="AppMirror destination SC name"
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
def static_app(request):
    return request.config.getoption("--static_app")


@pytest.fixture(scope="session")
def src_sc(request):
    return request.config.getoption("--src_sc")


@pytest.fixture(scope="session")
def dst_sc(request):
    return request.config.getoption("--dst_sc")


# ---------------
# End Arg Parse #
# ---------------


@pytest.fixture(scope="session")
def s3_creds(s3_host, s3_access_key, s3_secret_key) -> namedtuple:
    S3_Creds = namedtuple('S3Creds', 'host access_key secret_key')
    return S3_Creds(host=s3_host, access_key=s3_access_key, secret_key=s3_secret_key)


@pytest.fixture(scope="session")
def bucket_manager(s3_creds):
    bucket_manager = BucketManager(s3_creds.host, s3_creds.access_key, s3_creds.secret_key)
    yield bucket_manager
    bucket_manager.cleanup_buckets()


@pytest.fixture(scope="session")
def app_cluster(kubeconfig, bucket_manager) -> Cluster:
    logger.info(f"Using kubeconfig: {kubeconfig}")
    default_test_bucket = bucket_manager.create_bucket(f"test-bucket-{random.get_short_uuid()}")
    cluster = Cluster(kubeconfig, default_test_bucket)
    yield cluster

    # Cluster cleanup, runs after all tests are complete
    cluster.cleanup()


# If static_app = True - we expect there to already be an app created in the 'testapp' NS.
# static_app is useful for quick iteration while developing
@pytest.fixture(scope="session")
def default_app(app_cluster, static_app: bool) -> App:
    # Note: default_app uses the default SC
    if not static_app:
        name = "mariadb"
        namespace = f"maria-testapp-{random.get_short_uuid()}"
        return app_cluster.app_installer.install_mariadb(name, namespace)
    else:
        name = "testapp"
        namespace = "testapp"
        return App(
            name=name,
            namespace=namespace,
            k8s_helper=app_cluster.k8s_helper
        )


@pytest.fixture(scope="session")
def default_app_vault(app_cluster):
    secret_name = f"app-vault-secret-{random.get_short_uuid()}"
    app_cluster.k8s_helper.create_secretkey_accesskey_secret(
        namespace=defaults.CONNECTOR_NAMESPACE,
        secret_name=secret_name,
        access_key=app_cluster.default_test_bucket.access_key,
        secret_key=app_cluster.default_test_bucket.secret_key
    )

    app_vault_name = f"test-app-vault-{random.get_short_uuid()}"
    cr_response = app_cluster.app_vault_helper.apply_cr(
        cr_name=app_vault_name,
        namespace=defaults.CONNECTOR_NAMESPACE,
        bucket_name=app_cluster.default_test_bucket.bucket_name,
        bucket_host=app_cluster.default_test_bucket.host,
        secret_name=secret_name,
        provider_type="generic-s3"
    )
    return cr_response


# Uses function scope so each appmirror test gets its own app and the app gets cleaned up after each test
@pytest.fixture(scope="function")
def appmirror_src_app_fixture_scope(app_cluster, src_sc):
    # Note: default_app uses the default SC
    name = "appmirror-test-app"
    namespace = f"appmirror-test-{random.get_short_uuid()}"
    app = app_cluster.app_installer.install_mariadb(name, namespace, storage_class=src_sc)
    yield app
    app.uninstall()
