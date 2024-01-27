import k8s_helper
import buckets
import base64

class AppVault:
    def __init__(self, name):
        self.name = name


class AppVaultManager:
    created_app_vaults = []

    def __init__(self, k8s_helper: k8s_helper.K8sHelper, bucket_manager: buckets.BucketManager):
        self.k8s_helper = k8s_helper
        self.bucket_manager = bucket_manager

    @staticmethod
    def get_app_vault_cr(name, endpoint, bucket_name, secret_name, provider_type="generic-s3"):
        return {
            "apiVersion": "astra.netapp.io/v1",
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

    def create_app_vault(self, name):
        app_vault_def = self.get_app_vault_cr(name, self.bucket_manager.host, self.bucket_manager.)


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