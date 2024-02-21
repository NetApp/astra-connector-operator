from kubernetes.client import ApiException
import python_tests.defaults as defaults
from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper


class AppVaultHelper:
    created_app_vaults: list[dict] = []
    plural_name = "appvaults"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_cr(self, name, endpoint, bucket_name, secret_name, provider_type="generic-s3", secure=False):
        return {
            "apiVersion": f"{self.group}/{self.version}",
            "kind": "AppVault",
            "metadata": {
                "name": name,
            },
            "spec": {
                "providerType": provider_type,
                "providerConfig": {
                    "endpoint": endpoint,
                    "bucketName": bucket_name,
                    "secure": str(secure).lower(),
                },
                "providerCredentials": {
                    "accessKeyID": {
                        "valueFromSecret": {
                            "name": secret_name,
                            "key": "accessKeyID",
                        }
                    },
                    "secretAccessKey": {
                        "valueFromSecret": {
                            "name": secret_name,
                            "key": "secretAccessKey"
                        }
                    },
                },
            },
        }

    def apply_cr(self, name, bucket_name, bucket_host, secret_name,
                 provider_type="generic-s3", namespace=defaults.CONNECTOR_NAMESPACE) -> dict:
        app_vault_def = self.gen_cr(name, bucket_host, bucket_name, secret_name, provider_type)
        cr_response = self.k8s_helper.apply_cr(name, namespace, app_vault_def, self.plural_name)
        self.created_app_vaults.append(cr_response)
        return cr_response

    def get_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def delete_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def cleanup(self):
        for app_vaults in self.created_app_vaults:
            try:
                name = app_vaults.get('metadata', {}).get('name', '')
                namespace = app_vaults.get('metadata', {}).get('namespace', '')
                self.delete_cr(name=name, namespace=namespace)
            except ApiException as e:
                # Don"t fail if the CR has already been deleted
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up secrets: {e}")
