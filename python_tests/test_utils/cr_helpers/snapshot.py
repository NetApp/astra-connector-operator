from python_tests.test_utils.k8s_helper import K8sHelper
import python_tests.defaults as defaults


class SnapshotHelper:
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
        return self.k8s_helper.apply_cr(name, namespace, snapshot_def, self.plural_name)

    def get_cr(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)
