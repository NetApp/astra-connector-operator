# Astra Connector Operator 

Astra Control Service uses Astra Connector to enable communication between Astra Control Service and private clusters. You need to install Astra Connector on private clusters that you want to manage.

Astra Connector supports the following types of private clusters:

- Amazon Elastic Kubernetes Service (EKS)
- Azure Kubernetes Service (AKS)
- Google Kubernetes Engine (GKE)
- Red Hat OpenShift Service on AWS (ROSA)
- ROSA with AWS PrivateLink
- Red Hat OpenShift Container Platform on-premise

## About this task

When you perform these steps, execute these commands against the private cluster that you want to manage with Astra Control Service.

If you are using a bastion host, issue these commands from the command line of the bastion host.

**Note:** ROSA clusters only: After you install Astra Connector on your ROSA cluster, the cluster is automatically added to Astra Control Service.

## Before you begin

You need access to the private cluster you want to manage with Astra Control Service.

You need Kubernetes administrator permissions to install the Astra Connector operator on the cluster.

## Steps

1. Install the Astra Connector operator on the private cluster you want to manage with Astra Control Service. When you run this command, the namespace `astra-connector-operator` is created and the configuration is applied to the namespace:

    ```bash
    kubectl apply -f astraconnector_operator.yaml
    ```

2. Verify that the operator is installed and ready:

    ```bash
    kubectl get all -n astra-connector-operator
    ```

3. Get an API token from Astra Control. Refer to the Astra Automation documentation for instructions.

4. Update the Astra Connector CR file located at `controllerconfig.yaml`. Update the values in brackets <> to match your Astra Control environment and cluster configuration:

    ```yaml
    apiVersion: netapp.astraconnector.com/v1
    kind: AstraConnector
    metadata:
      name: astra-connector
      namespace: astra-connector
    spec:
      natssync-client:
        image: natssync-client:2.0.202302011758
        cloud-bridge-url: <ASTRA_CONTROL_SERVICE_URL>
      nats:
        image: nats:2.6.1-alpine3.14
      httpproxy-client:
        image: httpproxylet:2.2.202310111117
      echo-client:
        image: echo-proxylet:2.0
      imageRegistry:
        name: theotw
        secret: otw-secret
      astra:
        token: <ASTRA_CONTROL_SERVICE_API_TOKEN>
        #clusterName: <PRIVATE_AKS_CLUSTER_NAME>
        accountId: <ASTRA_CONTROL_ACCOUNT_ID>
        acceptEULA: yes
    ```

5. After you populate the `controllerconfig.yaml` file with the correct values, apply the CR:

    ```bash
    kubectl apply -f controllerconfig.yaml
    ```

6. Verify that the Astra Connector is fully deployed:

    ```bash
    kubectl get all -n astra-connector
    ```

7. Verify that the cluster is registered with Astra Control:

    ```bash
    kubectl get astraconnectors.astra.netapp.io -A
    ```

You should see output similar to the following:

```bash
NAMESPACE         NAME              REGISTERED   ASTRACONNECTORID                       STATUS
neptune-system   astra-connector   true         00a821c8-2cef-41ac-8777-ed05a417883e   Registered with Astra