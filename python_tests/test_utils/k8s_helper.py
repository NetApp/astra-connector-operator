import base64
import python_tests.defaults as defaults
from kubernetes import client, config, utils
from kubernetes.client import ApiException, V1Status, V1Secret
from python_tests.log import logger


class K8sHelper:
    created_secrets: list[V1Secret] = []

    def __init__(self, kubeconfig):
        self.kubeconfig = kubeconfig
        self.api_client = config.new_client_from_config(config_file=self.kubeconfig)
        self.core_v1_api = client.CoreV1Api(self.api_client)
        self.custom_object_api = client.CustomObjectsApi(self.api_client)

    def create_cr(self, namespace, body, plural) -> dict:
        group, version = body['apiVersion'].split('/')
        return self.custom_object_api.create_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            body=body,
        )

    def update_cr(self, namespace, name, body, plural) -> dict:
        group, version = body['apiVersion'].split('/')
        return self.custom_object_api.patch_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            name=name,
            body=body,
        )

    def delete_cr(self, namespace, name, group, version, plural) -> V1Status:
        return self.custom_object_api.delete_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            name=name,
        )

    def apply_cr(self, name, namespace, body, plural) -> dict:
        group, version = body['apiVersion'].split('/')
        try:
            self.custom_object_api.get_namespaced_custom_object(
                group=group,
                version=version,
                namespace=namespace,
                plural=plural,
                name=name,
            )
            return self.update_cr(namespace, name, body, plural)
        except ApiException as e:
            if e.status == 404:
                return self.create_cr(namespace, body, plural)
            else:
                raise

    def get_cr(self, name, namespace, group, version, plural) -> dict:
        return self.custom_object_api.get_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            name=name,
        )

    def create_from_file(self, file_path) -> list[list]:
        # Create resources from the YAML file. NOTE: will error if a resource already exists
        return utils.create_from_yaml(self.api_client, file_path)

    def create_secret(self, namespace, body) -> V1Secret:
        secret = self.core_v1_api.create_namespaced_secret(
            namespace=namespace,
            body=body,
        )
        self.created_secrets.append(secret)
        return secret

    def get_secret(self, name, namespace) -> V1Secret:
        return self.core_v1_api.read_namespaced_secret(name=name,
                                                       namespace=namespace)

    def create_secretkey_accesskey_secret(self, secret_name, access_key, secret_key,
                                          namespace=defaults.DEFAULT_CONNECTOR_NAMESPACE) -> V1Secret:
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
        return self.create_secret(namespace, secret_def)

    def cleanup_created_secrets(self):
        for secret in self.created_secrets:
            try:
                name = secret.metadata.name
                namespace = secret.metadata.namespace
                if name == '' or namespace == '':
                    continue
                self.core_v1_api.delete_namespaced_secret(name=name, namespace=namespace)
            except ApiException as e:
                # If the Secret was not found log and continue, we don"t want to fail due to clean up
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up secrets: {e}")

