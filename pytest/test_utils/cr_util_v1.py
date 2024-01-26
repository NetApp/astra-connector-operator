"""todo: comment this file"""
import uuid


class CrUtilV1:

    # Common variables are stored in the class to reduce needed parameters per method
    def __init__(self, image_registry, registry_secret_name):
        self.image_registry = image_registry
        self.registry_secret_name = registry_secret_name

    def get_connector_operator_cr_filepath(self):
        #todo
        pass

    def get_connector_cr(self, cluster_name, cloud_bridge_url, host_alias_ip=None, account_id=f"mock-{uuid.uuid4()[:8]}",
                            namespace="astra-connector", name="astra-connector", skip_tls_validation="true",
                            api_token_secret_name="astra-token"):
        return {
            "apiVersion": "astra.netapp.io/v1",
            "kind": "AstraConnector",
            "metadata": {
                "name": name,
                "namespace": namespace,
            },
            "spec": {
                "astra": {
                    "accountId": account_id,
                    "clusterName": cluster_name,
                    "skipTLSValidation": skip_tls_validation,
                    "tokenRef": api_token_secret_name,
                },
                "natsSyncClient": {
                    "cloudBridgeURL": cloud_bridge_url,
                    "hostAliasIP": host_alias_ip,
                },
                "imageRegistry": {
                    "name": self.image_registry,
                    "secret": self.registry_secret_name,
                },
            },
        }


    def get_app_vault_cr(self, name, endpoint, bucket_name, access_key_secret_name, access_key_secret_key,
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
