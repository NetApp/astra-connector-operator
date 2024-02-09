import uuid
from python_tests import defaults


def test_create_app_vault(app_clusters):
    secret_name = f"app-vault-secret-{str(uuid.uuid4())[:8]}"
    app_clusters[0].cr_helper.app_vault.create_app_vault_secret(
        namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE,
        secret_name=secret_name,
        access_key=app_clusters[0].default_test_bucket.access_key,
        secret_key=app_clusters[0].default_test_bucket.secret_key
    )

    app_vault_name = f"test-app-vault-{str(uuid.uuid4())[:8]}"
    app_clusters[0].cr_helper.app_vault.create_app_vault(
        name=app_vault_name,
        namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE,
        bucket_name=app_clusters[0].default_test_bucket.bucket_name,
        bucket_host=app_clusters[0].default_test_bucket.host,
        secret_name=secret_name,
        provider_type="generic-s3"
    )


def test_app_snapshot(shared_app_vault):
    pass
