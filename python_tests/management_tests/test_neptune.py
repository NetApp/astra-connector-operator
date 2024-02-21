import io
import python_tests.test_utils.random as random
from python_tests import defaults, constants

# --- POC TESTS ---


# For POC only
# How to install/uninstall an app
def test_create_mariadb_app(app_cluster):
    # Install App
    namespace = f"mariadb-test-{random.get_short_uuid()}"
    app = app_cluster.app_installer.install_mariadb("test-app", namespace)
    try:
        # Get app's pods
        pods = app_cluster.k8s_helper.get_pods(app.namespace)
        for pod in pods.items:
            print(f"Found pod: {pod.metadata.name}")
    finally:
        # Uninstall App
        app.uninstall()


# For POC only
# How to create and delete buckets on the fly
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


# POC only
def test_create_app_vault_secret(app_cluster):
    secret_name = f"app-vault-test-{random.get_short_uuid()}"
    app_cluster.k8s_helper.create_secretkey_accesskey_secret(
        secret_name=secret_name,
        access_key=app_cluster.default_test_bucket.access_key,
        secret_key=app_cluster.default_test_bucket.secret_key,
        namespace=defaults.CONNECTOR_NAMESPACE
    )

    # Verify secret exists
    secret = app_cluster.k8s_helper.get_secret(name=secret_name, namespace=defaults.CONNECTOR_NAMESPACE)
    assert secret.metadata.name == secret_name, f"secret {secret_name} not found"


def test_create_app_vault(app_cluster):
    mock_secret_name = "mock-secret"
    app_vault_name = f"test-app-vault-{random.get_short_uuid()}"
    app_cluster.app_vault.apply_cr(
        cr_name=app_vault_name,
        bucket_name=app_cluster.default_test_bucket.bucket_name,
        bucket_host=app_cluster.default_test_bucket.host,
        secret_name=mock_secret_name
    )

    # Get app vaults from kubernetes and assert it's there
    app_vault = app_cluster.app_vault.get_cr(app_vault_name)
    assert app_vault['metadata']['name'] == app_vault_name, "failed to find app vault cr after creation"


def test_app_snapshot(app_cluster, default_app_vault, default_app):
    # Create application CR
    app_name = f"{default_app.name}-{random.get_short_uuid()}"
    app_cluster.application_helper.apply_cr(
        namespace=defaults.CONNECTOR_NAMESPACE,
        cr_name=app_name,
        included_namespaces=[default_app.namespace]
    )

    # Create snapshot CR
    snapshot_name = f"test-snap-{random.get_short_uuid()}"
    app_vault_name = default_app_vault['metadata']['name']
    app_cluster.snapshot_helper.apply_cr(cr_name=snapshot_name, application_name=app_name,
                                         app_vault_name=app_vault_name)

    app_cluster.snapshot_helper.wait_for_snapshot_with_timeout(snapshot_name, timeout_sec=120)
    state = app_cluster.snapshot_helper.get_cr(snapshot_name)['status'].get("state", "")
    assert state.lower() == "completed", f"expected snapshot '{snapshot_name}' state 'completed' but got '{state}'"


def test_backup(app_cluster, default_app_vault, default_app):
    # Create App CR
    app_name = f"{default_app.name}-{random.get_short_uuid()}"
    app_cluster.application_helper.apply_cr(app_name, [default_app.namespace])

    # Create Snapshot CR
    snap_name = f"snapshot-test-{random.get_short_uuid()}"
    app_vault_name = default_app_vault['metadata']['name']
    app_cluster.snapshot_helper.apply_cr(snap_name, app_name, app_vault_name)

    # Wait for snapshot to complete
    app_cluster.snapshot_helper.wait_for_snapshot_with_timeout(snap_name, timeout_sec=120)
    state = app_cluster.snapshot_helper.get_cr(snap_name)['status'].get("state", "")
    assert state.lower() == "completed", f"expected snapshot '{snap_name}' state 'completed' but got '{state}'"

    # Create Backup CR
    backup_name = f"backup-test-{random.get_short_uuid()}"
    app_cluster.backup_helper.apply_cr(backup_name, app_name, snap_name, app_vault_name)

    # Wait for backup to complete
    app_cluster.backup_helper.wait_for_snapshot_with_timeout(backup_name, timeout_sec=300)
    state = app_cluster.backup_helper.get_cr(backup_name)['status'].get("state", "")
    assert state.lower() == "completed", f"timed out waiting for backup {backup_name} to complete"


def test_appmirror_establish_promote(app_cluster, default_app_vault, appmirror_src_app_fixture_scope, dst_sc):
    # Use the default app vault for both src and dest
    src_app_vault = default_app_vault
    dest_app_vault = default_app_vault
    src_app = appmirror_src_app_fixture_scope

    # Create src app CR
    app_name = f"{src_app.name}-{random.get_short_uuid()}"
    app_cluster.application_helper.apply_cr(cr_name=app_name, included_namespes=[src_app.namespace])

    # Create Snapshot CR
    snap_name = f"snapshot-test-{random.get_short_uuid()}"
    app_vault_name = default_app_vault['metadata']['name']
    app_cluster.snapshot_helper.apply_cr(snap_name, app_name, app_vault_name)

    # Wait for snapshot to complete
    app_cluster.snapshot_helper.wait_for_snapshot_with_timeout(snap_name, timeout_sec=120)
    snap_cr = app_cluster.snapshot_helper.get_cr(snap_name)
    state = snap_cr['status'].get("state", "")
    assert state.lower() == "completed", f"expected snapshot '{snap_name}' state 'completed' but got '{state}'"

    # Create src schedule CR
    schedule_name = f"schedule-{random.get_short_uuid()}"
    app_cluster.schedule_helper.apply_cr(
        cr_name=schedule_name,
        app_name=app_name,
        app_vault_name=src_app_vault['metadata']['name'],
        replicate=True,
        enabled=True,
        interval=5,
        frequency=constants.Frequency.MINUTELY.value
    )

    # Create dest app CR
    dest_app_ns = f"appmirror-dest-{random.get_short_uuid()}"
    dest_app_name = f"appmirror-dest-{random.get_short_uuid()}"
    dest_app_cr = app_cluster.application_helper.apply_cr(dest_app_name, [dest_app_ns])

    try:
        # Create appmirror CR
        snap_path = app_cluster.snapshot_helper.get_snap_path(snap_cr)
        am_name = f"appmirror-{random.get_short_uuid()}"
        app_cluster.appmirror_helper.apply_cr(
            cr_name=am_name,
            app_name=dest_app_cr['metadata']['name'],
            desired_state=constants.AppmirrorState.ESTABLISHED.value,
            src_app_vault_name=src_app_vault['metadata']['name'],
            dest_app_vault_name=dest_app_vault['metadata']['name'],
            snap_path=snap_path,
            sc_name=dst_sc,
            ns_mapping=[{
                "source": src_app.namespace,
                "destination": dest_app_ns
            }]
        )

        # Wait for appmirror to be in established state
        app_cluster.appmirror_helper.wait_for_state_with_timeout(
            cr_name=am_name,
            appmirror_state=constants.AppmirrorState.ESTABLISHED.value,
            timeout_sec=300
        )
        cr = app_cluster.appmirror_helper.get_cr(am_name)
        appmirror_state = cr.get('status', {}).get("state", "")
        assert appmirror_state == constants.AppmirrorState.ESTABLISHED.value, \
            f"timed out waiting for appmirror state '{constants.AppmirrorState.ESTABLISHED.value}'\n{cr}"

        # Update CR to desired state "promoted"
        cr['spec']['desiredState'] = constants.AppmirrorState.PROMOTED.value
        app_cluster.appmirror_helper.update_cr(cr['metadata']['name'], cr)
        # Wait for appmirror to be in promoted state
        app_cluster.appmirror_helper.wait_for_state_with_timeout(
            cr_name=am_name,
            appmirror_state=constants.AppmirrorState.PROMOTED.value,
            timeout_sec=300
        )
        cr = app_cluster.appmirror_helper.get_cr(am_name)
        appmirror_state = cr.get('status', {}).get("state", "")
        assert appmirror_state == constants.AppmirrorState.PROMOTED.value, \
            f"timed out waiting for appmirror state '{constants.AppmirrorState.PROMOTED.value}'\n{cr}"

    finally:
        app_cluster.k8s_helper.delete_namespace(dest_app_ns)
