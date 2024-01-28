import pytest
import uuid
from python_tests import config


def test_create_app_vault(cr_helper, shared_bucket):
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


def test_app_snapshot(shared_app_vault):
    pass
