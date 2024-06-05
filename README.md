## This branch is currently being used for integration work. If you want to use the released build, please visit [this branch](https://github.com/NetApp/astra-connector-operator/tree/release-23-07) and follow the documentation there instead.

## Install the Astra Connector

This guide provides instructions for installing the latest version of the Astra Connector on your private cluster. If you need to install a specific release, refer to the appropriate release bundle. If you are using a bastion host, execute these commands from the command line of the bastion host.

### Steps

1. Apply the Astra Connector operator. When you run this command, the correct namespace for the Astra Connector is created and the configuration is applied to the namespace:

    ```bash
    kubectl apply -f https://github.com/NetApp/astra-connector-operator/releases/latest/download/astraconnector_operator.yaml
    ```

2. Verify that the operator is installed and ready:

    ```bash
    kubectl get all -n astra-connector-operator
    ```

3. Generate an Astra Control API token using the instructions in the [Astra Automation documentation](https://docs.netapp.com/us-en/astra-automation/get-started/get_api_token.html).

4. Create a secret using the token. Replace `<API_TOKEN>` with the token you received from Astra Control:

    ```bash
    kubectl create secret generic astra-token \
    --from-literal=apiToken=<API_TOKEN> \
    -n astra-connector
    ```

5. Create a Docker secret to use to pull the Astra Connector image. Replace values in brackets <> with information from your environment:

    ```bash
    kubectl create secret docker-registry regcred \
    --docker-username=<ASTRA_ACCOUNT_ID> \
    --docker-password=<API_TOKEN> \
    -n astra-connector \
    --docker-server=cr.astra.netapp.io
    ```

6. Create the Astra Connector CR file and name it `astra-connector-cr.yaml`. Update the values in brackets <> to match your Astra Control environment and cluster configuration:

    ```yaml
    apiVersion: astra.netapp.io/v1
    kind: AstraConnector
    metadata:
      name: astra-connector
      namespace: astra-connector
    spec:
      astra:
        accountId: <ASTRA_ACCOUNT_ID>
        clusterName: <CLUSTER_NAME>
        skipTLSValidation: false  # Should be set to false in production environments
        tokenRef: astra-token
        astraControlURL: <ASTRA_CONTROL_HOST_URL>
        hostAliasIP: <ASTRA_HOST_ALIAS_IP_ADDRESS>
      imageRegistry:
        name: cr.astra.netapp.io/astra
        secret: regcred
    ```

7. Apply the `astra-connector-cr.yaml` file after you populate it with the correct values:

    ```bash
    kubectl apply -n astra-connector -f astra-connector-cr.yaml
    ```

8. Verify that the Astra Connector is fully deployed:

    ```bash
    kubectl get all -n astra-connector
    ```

9. Verify that the cluster is registered with Astra Control:

    ```bash
    kubectl get astraconnectors.astra.netapp.io -A
    ```

   You should see output similar to the following:

    ```
    NAMESPACE         NAME              REGISTERED   ASTRACONNECTORID                       STATUS
    astra-connector   astra-connector   true         00a821c8-2cef-41ac-8777-ed05a417883e   Registered with Astra
    ```
