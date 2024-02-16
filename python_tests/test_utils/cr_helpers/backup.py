import python_tests.defaults as defaults
from python_tests.test_utils.k8s_helper import K8sHelper
from kubernetes.client import ApiException
from python_tests.log import logger
import time


class BackupHelper:
    created_backups: list[dict] = []
    plural_name = "backups"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_cr(self, name, application_name, snapshot_name, app_vault_name) -> dict:
        return {
            "apiVersion": f"{self.group}/{self.version}",
            "kind": "Backup",
            "metadata": {
                "name": name,
            },
            "spec": {
                "applicationRef": application_name,
                "snapshotRef": snapshot_name,
                "appVaultRef": app_vault_name
            }
        }

    def apply_cr(self, name, application_name, snapshot_name, app_vault_name,
                 namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE) -> dict:
        cr_def = self.gen_cr(name, application_name, snapshot_name, app_vault_name)
        cr_response = self.k8s_helper.apply_cr(name, namespace, cr_def, self.plural_name)
        self.created_backups.append(cr_response)
        return cr_response

    def get_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def delete_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def cleanup(self):
        for backup in self.created_backups:
            try:
                name = backup.get('metadata', {}).get('name', '')
                namespace = backup.get('metadata', {}).get('name', '')
                if name == '' or namespace == '':
                    continue
                self.delete_cr(name=name, namespace=namespace)
            except ApiException as e:
                # Don"t fail if the CR has already been deleted
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up backup CRs: {e}")

    def wait_for_snapshot_with_timeout(self, name, timeout_sec, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        time_expire = time.time() + timeout_sec
        while time.time() < time_expire:
            state = self.get_cr(name)['status'].get("state", "")
            if state.lower() == "completed":
                return
            if state.lower() == "error":
                raise Exception(f"backup {name} is in state {state}")

        raise TimeoutError(f"Timed out waiting for backup {name} to complete")
