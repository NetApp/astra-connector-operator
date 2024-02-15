import python_tests.defaults as defaults
from python_tests.test_utils.k8s_helper import K8sHelper


class ApplicationHelper:
    plural_name = "applications"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_application_cr(self, name, included_namespaces: list[str]) -> dict:
        return {
            "apiVersion": f"{self.group}/{self.version}",
            "kind": "Application",
            "metadata": {
                "name": name,
            },
            "spec": {
                "includedNamespaces": [{"namespace": ns} for ns in included_namespaces],
            }
        }

    def apply_application_cr(self, name, included_namespaces: list[str],
                             namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE) -> dict:
        snapshot_def = self.gen_application_cr(name, included_namespaces)
        return self.k8s_helper.apply_cr(name, namespace, snapshot_def, self.plural_name)


