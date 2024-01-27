import k8s_helper
import buckets

class AppVault:
    def __init__(self, name):
        self.name = name


class AppVaultManager:
    created_app_vaults = []

    def __init__(self, k8s_helper: k8s_helper.K8sHelper, bucket_manager: buckets.BucketManager):
        self.k8s_helper = k8s_helper
        self.bucket_manager = bucket_manager

    @staticmethod
    def get_app_vault_cr(name, endpoint, bucket_name, access_key_secret_name, access_key_secret_key,
                         secret_access_key_secret_name, secret_access_key_secret_key, provider_type):
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
                            "name": access_key_secret_name,
                            "key": access_key_secret_key,
                        }
                    },
                    "secretAccessKey": {
                        "valueFromSecret": {
                            "name": secret_access_key_secret_name,
                            "key": secret_access_key_secret_key,
                        }
                    },
                },
            },
        }

    def create_app_vault(self, name):
        app_vault_def = self.get_app_vault_cr(name, self.bucket_manager.host, self.bucket_manager.)


    def create_app_vault_secret(self, secret_name, value):
        pass
        # self.k8s_helper.create_secret()