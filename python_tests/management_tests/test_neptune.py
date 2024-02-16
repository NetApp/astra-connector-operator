import io

import python_tests.test_utils.random as random
from python_tests import defaults
from python_tests.test_utils.app_installer import App


# For POC only
def test_create_mariadb(app_cluster, default_app):
    pods = app_cluster.k8s_helper.get_pods(default_app.namespace)
    for pod in pods.items:
        print(f"found pod: {pod.metadata.name}")


# For POC only
# Buckets on the fly
def test_bucket_create_read_write_delete(bucket_manager):
    # Create a bucket
    bucket = bucket_manager.create_bucket(f"example-create-bucket-{random.get_short_uuid()}")

    data = b"Hello, World!"
    object_name = "test-object"
    try:
        # Write to bucket
        bucket_manager.client.put_object(bucket.bucket_name, object_name, io.BytesIO(data), len(data))

        # Read
        response = bucket_manager.client.get_object(bucket.bucket_name, object_name)
        read_data = response.read()

        # Use assert to compare the written data with the read data
        assert data == read_data, f"Data mismatch: expected: '{data}', got: '{read_data}'"
    finally:
        # Cleanup the bucket we created
        bucket_manager.delete_object(bucket.bucket_name, object_name)
        bucket_manager.delete_bucket(bucket.bucket_name)


def test_create_app_vault_secret(app_cluster):
    secret_name = f"app-vault-test-{random.get_short_uuid()}"
    app_cluster.k8s_helper.create_secretkey_accesskey_secret(
        secret_name=secret_name,
        access_key=app_cluster.default_test_bucket.access_key,
        secret_key=app_cluster.default_test_bucket.secret_key,
        namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE
    )

    # Verify secret exists
    secret = app_cluster.k8s_helper.get_secret(name=secret_name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE)
    assert secret.metadata.name == secret_name, f"secret {secret_name} not found"


def test_create_app_vault(app_cluster):
    mock_secret_name = "mock-secret"
    app_vault_name = f"test-app-vault-{random.get_short_uuid()}"
    app_cluster.app_vault.apply_cr(
        name=app_vault_name,
        bucket_name=app_cluster.default_test_bucket.bucket_name,
        bucket_host=app_cluster.default_test_bucket.host,
        secret_name=mock_secret_name
    )

    # Get app vaults from kubernetes and assert it's there
    app_vault = app_cluster.app_vault.get_cr(app_vault_name)
    assert app_vault['metadata']['name'] == app_vault_name, "failed to find app vault cr after creation"


def test_app_snapshot(app_cluster, default_app_vault: dict):
    default_app = App(
        namespace="maria1",
        name="maria"
    )
    # Create application CR
    app_cluster.application_helper.apply_cr(
        namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE,
        name=default_app.name,
        included_namespaces=[default_app.namespace]
    )

    # Create snapshot CR
    snapshot_name = f"test-snap-{random.get_short_uuid()}"
    app_vault_name = default_app_vault['metadata']['name']
    app_cluster.snapshot_helper.apply_cr(name=snapshot_name, application_name=default_app.name,
                                         app_vault_name=app_vault_name)

    app_cluster.snapshot_helper.wait_for_snapshot_with_timeout(snapshot_name, timeout_sec=60)
    state = app_cluster.snapshot_helper.get_cr(snapshot_name)['status'].get("state", "")
    assert state.lower() == "completed", f"expected snapshot '{snapshot_name}' state 'completed' but got '{state}'"


def test_backup(app_cluster, default_app_vault):
    default_app = App(
        namespace="maria1",
        name="maria"
    )
    # Create App CR
    app_cluster.application_helper.apply_cr(default_app.name, [default_app.namespace])

    # Create Snapshot CR
    snap_name = f"snapshot-test-{random.get_short_uuid()}"
    app_vault_name = default_app_vault['metadata']['name']
    app_cluster.snapshot_helper.apply_cr(snap_name, default_app.name, app_vault_name)

    # Wait for snapshot to complete
    app_cluster.snapshot_helper.wait_for_snapshot_with_timeout(snap_name, timeout_sec=60)
    state = app_cluster.snapshot_helper.get_cr(snap_name)['status'].get("state", "")
    assert state.lower() == "completed", f"expected snapshot '{snap_name}' state 'completed' but got '{state}'"

    # Create Backup CR
    backup_name = f"backup-test-{random.get_short_uuid()}"
    app_cluster.backup_helper.apply_cr(backup_name, default_app.name, snap_name, app_vault_name)

    # Wait for backup to complete
    app_cluster.backup_helper.wait_for_snapshot_with_timeout(backup_name, timeout_sec=300)
    state = app_cluster.backup_helper.get_cr(backup_name)['status'].get("state", "")
    assert state.lower() == "completed", f"timed out waiting for backup {backup_name} to complete"
