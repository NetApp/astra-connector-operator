from k8s_helper import K8sHelper
from python_tests import config
import uuid

#todo rename without helper
class AstraConnectorHelper:

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def get_connector_cr(self, cluster_name, cloud_bridge_url, host_alias_ip=None, account_id=f"mock-{uuid.uuid4()[:8]}",
                         namespace=config.DEFAULT_CONNECTOR_NAMESPACE, name="astra-connector", skip_tls_validation="true",
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

    def create_astra_connector(self):
        pass


