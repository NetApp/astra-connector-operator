from python_tests.test_utils.k8s_helper import K8sHelper
import base64


# Only needs name and namespace, just enough to make K8s API calls
class AppVault:
    def __init__(self, name, namespace):
        self.name = name
        self.namespace = namespace


class AppVaultHelper:
    created_app_vaults: list[AppVault] = []
    plural_name = "appvaults"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def get_app_vault_cr(self, name, endpoint, bucket_name, secret_name, provider_type="generic-s3"):
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

    def create(self, name, namespace, bucket_name, bucket_host, secret_name, provider_type="generic-s3"):
        app_vault_def = self.get_app_vault_cr(name, bucket_host, bucket_name, secret_name, provider_type)
        self.k8s_helper.apply_cr(namespace, name, app_vault_def, self.plural_name)
        self.created_app_vaults.append(AppVault(name, namespace))

    def delete(self, name, namespace):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def create_app_vault_secret(self, namespace, secret_name, access_key, secret_key):
        access_key_encoded = base64.b64encode(access_key.encode()).decode()
        secret_key_encoded = base64.b64encode(secret_key.encode()).decode()
        secret_def = {
            "apiVersion": "v1",
            "kind": "Secret",
            "metadata": {
                "name": secret_name,
                "namespace": namespace,
            },
            "type": "Opaque",
            "data": {
                "accessKeyID": access_key_encoded,
                "secretAccessKey": secret_key_encoded,
            },
        }
        self.k8s_helper.create_secret(namespace, secret_def)

    # Useful for cleaning up after returning from yield in fixtures
    def cleanup_all_created_appvaults(self):
        for app_vault in self.created_app_vaults:
            # todo need to skip delete or handle err if already deleted
            self.delete(app_vault.name, app_vault.namespace)
