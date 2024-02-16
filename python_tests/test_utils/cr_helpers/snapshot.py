from kubernetes.client import ApiException
import time

import python_tests.defaults as defaults
from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper


class SnapshotHelper:
    created_snapshot_crs: list[dict] = []
    plural_name = "snapshots"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_cr(self, name, application_name, app_vault_name) -> dict:
        return {
            "apiVersion": f"{self.group}/{self.version}",
            "kind": "Snapshot",
            "metadata": {
                "name": name,
            },
            "spec": {
                "applicationRef": application_name,
                "appVaultRef": app_vault_name,
            }
        }

    def apply_cr(self, name, application_name, app_vault_name,
                 namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE) -> dict:
        snapshot_def = self.gen_cr(name, application_name, app_vault_name)
        cr = self.k8s_helper.apply_cr(name, namespace, snapshot_def, self.plural_name)
        self.created_snapshot_crs.append(cr)
        return cr

    def delete_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def get_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def cleanup(self):
        for snapshot in self.created_snapshot_crs:
            try:
                name = snapshot.get('metadata', {}).get('name', '')
                namespace = snapshot.get('metadata', {}).get('namespace', '')
                if name == '' or namespace == '':
                    continue
                self.delete_cr(name, namespace)
            except ApiException as e:
                # Don"t fail if the CR has already been deleted
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up snapshots: {e}")

    def wait_for_snapshot_with_timeout(self, name, timeout_sec, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        time_expire = time.time() + timeout_sec
        while time.time() < time_expire:
            state = self.get_cr(name)['status'].get("state", "")
            if state.lower() == "completed":
                return
            if state.lower() == "error":
                raise Exception(f"snapshot {name} is in state {state}")

        raise TimeoutError(f"Timed out waiting for snapshot {name} to complete")
