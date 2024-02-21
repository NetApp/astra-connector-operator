from kubernetes.client import V1Status, ApiException
import python_tests.defaults as defaults
from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper


class ApplicationHelper:
    created_applications: list[dict] = []
    plural_name = "applications"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_cr(self, name, included_namespaces: list[str]) -> dict:
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

    def apply_cr(self, cr_name, included_namespaces: list[str],
                 namespace=defaults.CONNECTOR_NAMESPACE) -> dict:
        cr_def = self.gen_cr(cr_name, included_namespaces)
        cr = self.k8s_helper.apply_cr(cr_name, namespace, cr_def, self.plural_name)
        self.created_applications.append(cr)
        return cr

    def delete_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE) -> V1Status:
        return self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def cleanup(self):
        for app in self.created_applications:
            try:
                name = app.get('metadata', {}).get('name', '')
                namespace = app.get('metadata', {}).get('namespace', '')
                self.delete_cr(name=name, namespace=namespace)
            except ApiException as e:
                # Don"t fail if the CR has already been deleted
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up application CRs: {e}")
