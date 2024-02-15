import base64
from python_tests.log import logger
from kubernetes.client import ApiException
import python_tests.defaults as defaults
from python_tests.test_utils.k8s_helper import K8sHelper


class AppVaultHelper:
    created_app_vaults: list[dict] = []
    created_secrets: list[dict] = []
    plural_name = "appvaults"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_app_vault_cr(self, name, endpoint, bucket_name, secret_name, provider_type="generic-s3", secure=False):
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

    def apply_app_vault(self, name, bucket_name, bucket_host, secret_name,
                        provider_type="generic-s3", namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE) -> dict:
        app_vault_def = self.gen_app_vault_cr(name, bucket_host, bucket_name, secret_name, provider_type)
        cr_response = self.k8s_helper.apply_cr(name, namespace, app_vault_def, self.plural_name)
        self.created_app_vaults.append(cr_response)
        return cr_response

    def get_app_vault(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def delete_app_vault(self, name, namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    # Useful for cleaning up after returning from yield in fixtures
    def cleanup_created_app_vaults(self):
        for app_vaults in self.created_app_vaults:
            try:
                name = app_vaults.get('metadata', {}).get('name', '')
                namespace = app_vaults.get('metadata', {}).get('name', '')
                if name == '' or namespace == '':
                    continue
                self.delete_app_vault(name=name, namespace=namespace)
            except ApiException as e:
                # If the Secret was not found log and continue, we don"t want to fail due to cleanup
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up secrets: {e}")

    def cleanup_created_secrets(self):
        for secret in self.created_secrets:
            try:
                name = secret.get('metadata', {}).get('name', '')
                namespace = secret.get('metadata', {}).get('name', '')
                if name == '' or namespace == '':
                    continue
                self.k8s_helper.core_v1_api.delete_namespaced_secret(name=name, namespace=namespace)
            except ApiException as e:
                # If the Secret was not found log and continue, we don"t want to fail due to cleanup
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up secrets: {e}")
