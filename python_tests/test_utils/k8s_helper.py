from kubernetes import client, config, utils


# K8sHelper todo comment. This is a top level classed used by other classes (or directly if needed) to
# manipulate CRs
class K8sHelper:
    def __init__(self, kubeconfig):
        self.api_client = config.new_client_from_config(config_file=kubeconfig)
        self.custom_object_api = client.CustomObjectsApi()

    def create_cr(self, namespace, body, plural):
        group, version = body['apiVersion'].split('/')
        return self.custom_object_api.create_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            body=body,
        )

    def update_cr(self, namespace, name, body, plural):
        group, version = body['apiVersion'].split('/')
        return self.custom_object_api.patch_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            name=name,
            body=body,
        )

    def delete_cr(self, namespace, name, group, version, plural):
        return self.custom_object_api.delete_namespaced_custom_object(
            group=group,
            version=version,
            namespace=namespace,
            plural=plural,
            name=name,
        )

    def apply_cr(self, namespace, name, body, plural):
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
        except self.api_client.rest.ApiException as e:
            if e.status == 404:
                return self.create_cr(namespace, body, plural)
            else:
                raise

    def create_from_file(self, file_path):
        # Create resources from the YAML file. NOTE: will error if a resource already exists
        utils.create_from_yaml(self.api_client, file_path)

    def create_secret(self, namespace, body):
        self.api_client.create_namespaced_secret(
            namespace=namespace,
            body=body,
        )
