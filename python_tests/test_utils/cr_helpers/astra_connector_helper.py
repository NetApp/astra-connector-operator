import uuid
from python_tests.test_utils.k8s_helper import K8sHelper
from python_tests import config


# Only needs name and namespace, just enough to be queried again via k8s client
class AstraConnector:
    def __init__(self, name, namespace):
        self.name = name
        self.namespace = namespace


# Helpers are tied to clusters/kubeconfig. It's expected to have two of these if you had two clusters.
class AstraConnectorHelper:
    plural_name = "astraconnectors"
    group = "astra.netapp.io"
    version = "v1"

    created_astra_connectors: list[AstraConnector] = []

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def get_connector_cr(self, cluster_name, cloud_bridge_url, image_registry, registry_secret_name, host_alias_ip=None,
                         account_id=f"mock-{uuid.uuid4()[:8]}", namespace=config.DEFAULT_CONNECTOR_NAMESPACE,
                         name="astra-connector", skip_tls_validation="true", api_token_secret_name="astra-token"):
        return {
            "apiVersion": f"{self.group}/{self.version}",
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
                    "name": image_registry,
                    "secret": registry_secret_name,
                },
            },
        }

    def create_astra_connector(self, cluster_name, cloud_bridge_url, image_registry, registry_secret_name,
                               host_alias_ip=None,
                               account_id=f"mock-{uuid.uuid4()[:8]}", namespace=config.DEFAULT_CONNECTOR_NAMESPACE,
                               name="astra-connector", skip_tls_validation="true", api_token_secret_name="astra-token"):
        cr_def = self.get_connector_cr(cluster_name=cluster_name,
                                       cloud_bridge_url=cloud_bridge_url,
                                       image_registry=image_registry,
                                       registry_secret_name=registry_secret_name,
                                       host_alias_ip=host_alias_ip,
                                       account_id=account_id,
                                       namespace=namespace,
                                       name=name,
                                       skip_tls_validation=skip_tls_validation,
                                       api_token_secret_name=api_token_secret_name)
        self.k8s_helper.apply_cr(namespace, name, cr_def, self.plural_name)
        # Only save if the CR was successfully applied
        self.created_astra_connectors.append(AstraConnector(name, namespace))

    # Useful for cleaning up after returning from yield in fixtures.
    # It is not expected to be more than one of these per AstraConnectorHelper/AppCluster instance,
    # but is technically possible
    def cleanup_all_created_astra_connectors(self):
        for connector in self.created_astra_connectors:
            # todo need to skip delete or handle err if already deleted
            self.k8s_helper.delete_cr(namespace=connector.namespace, name=connector.name, group=self.group,
                                      version=self.version, plural=self.plural_name)
