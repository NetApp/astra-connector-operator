from python_tests.test_utils.cr_helpers.astra_connector_helper import AstraConnectorHelper
from python_tests.test_utils.cr_helpers.app_vault_helper import AppVaultHelper
from k8s_helper import K8sHelper


class Cluster:

    # CrHelpers is to help organize helpers under the Cluster class and not meant to be used anywhere else
    class CrHelpers:
        def __init__(self, k8s_helper: K8sHelper):
            self.app_vault = AppVaultHelper(k8s_helper)
            self.astra_connector = AstraConnectorHelper(k8s_helper)

    def __init__(self, kubeconfig_path, default_bucket, default_app_vault):
        self.k8s_helper = K8sHelper(kubeconfig_path)
        self.cr_helper = self.CrHelpers(self.k8s_helper)
        self.default_test_bucket = default_bucket
        self.default_test_app_vault = default_app_vault

    def cleanup(self):
        self.cr_helper.app_vault.cleanup_all_created_appvaults()
        self.cr_helper.astra_connector.cleanup_all_created_astra_connectors()
