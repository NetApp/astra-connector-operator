from python_tests.test_utils.k8s_helper import K8sHelper
from python_tests.log import logger
import python_tests.defaults as defaults
from kubernetes.client import ApiException


class SnapshotHelper:
    created_snapshot_crs: list[dict] = []
    plural_name = "snapshots"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_snapshot_cr(self, name, application_name, app_vault_name) -> dict:
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

    def apply_snapshot_cr(self, name, application_name, app_vault_name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE) -> dict:
        snapshot_def = self.gen_snapshot_cr(name, application_name, app_vault_name)
        cr = self.k8s_helper.apply_cr(name, namespace, snapshot_def, self.plural_name)
        self.created_snapshot_crs.append(cr)
        return cr

    def delete_snapshot_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def get_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def cleanup_created_snapshot_crs(self):
        for snapshot in self.created_snapshot_crs:
            try:
                name = snapshot.get('metadata', {}).get('name', '')
                namespace = snapshot.get('metadata', {}).get('name', '')
                if name == '' or namespace == '':
                    continue
                self.delete_snapshot_cr(name=name, namespace=namespace)
            except ApiException as e:
                # If the Secret was not found log and continue, we don"t want to fail due to cleanup
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up secrets: {e}")
