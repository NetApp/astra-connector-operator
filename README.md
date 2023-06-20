# Astra Installer Operator 
Please refer to this confluence for now
https://confluence.ngage.netapp.com/display/POLARIS/Phase+1+%2823.07%29+Astra+Connector+Deployment+-+Non+VPN+Instructions

## Security Best Practices
### Use RBAC
Use RBAC (Role Based Access Control) in Kubernetes to limit access to the connector namespace after deployment

### Use a service mesh
Use a service mesh to secure communications between pods

### Limit token usage for registration
The API token used for registration is only used once, and is not used after registering the connector.
If the token was created specifically for registering the connector, we recommend discarding it after registration is complete.
Discarding the token requires removing the token from the AstraConnector CRD, and revoking it from Astra.

### Use a NetworkPolicy
Use a network policy to deny external communication to the pods in the connector namespace. The pods should only be making
outbound communication.

NOTE: A network policy requires a network policy controller to enforce the policy.

Here is an example NetworkPolicy to limit the communication:
```
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: astra-connector-network-policy
  namespace: astra-connector-namespace
spec:
  policyTypes:
  - Ingress
  - Egress
  // Only allow communication to pods from the connector namespace
  ingress:
  - from:
    namespaceSelector:
      name: astra-connector-operator
  // Allow all outgoing communication
  egress:
  - {}
```

## CRD
#### Sample CRD
```
apiVersion: astra/v1
kind: AstraConnector
metadata:
  name: astra-connector
spec:
  astra:
    token: <AstraApiToken>
    clusterName: ipsprintdemo
    accountId: 5831cf97-c52e-4185-ab83-db0fdd061c0e
    acceptEULA: yes
```

## CRD details
In the CRD, all the fields in the spec section have to be updated to the correct value

### imageRegistry
| CRD Spec          | Details       | Optional | Default |
| ----------------- | ------------- |--------- | --------|
| name   | Image registry to pull images from | Yes | dockerhub for nats, theotw for the rest |
| secret   | Image registry secret | Yes | "" |

Example:
```
spec:
    ...
    imageRegistry:
        name: theotw
        secret: otw-secret
```

### [natssync-client](https://github.com/theotw/natssync)
Natssync-Client talks to the Natssync-Server on Astra and ensures communication between Astra and the private AKS cluster

The natssync-client CRD is a map of key/value pairs

| CRD Spec          | Details       | Optional | Default                             |
| ----------------- | ------------- |--------- |-------------------------------------|
| image   | natssync-client image | Yes | natssync-client:2.0 |
| cloud-bridge-url  | Astra URL  | Yes | https://astra.netapp.io             |
| skipTLSValidation | Skip TLS Validation| Yes| false                               |
| hostalias | Use a custom IP for the cloud bridge hostname| Yes | false                               |
| hostaliasIP | IP to use for host alias | Yes if hostalias is false |                                     |

Example:
```
spec:
    ...
    natssync-client:
        image: theotw/natssync-client:0.9.202201132025
        cloud-bridge-url: https://integration.astra.netapp.io
```
### [nats](https://nats.io/)
| CRD Spec             | Details       | Optional | Default |
| ---------------------| ------------- |--------- | --------|
| size       | Replica count for the nats statefulset | Yes | 2 |
| image      | nats image | Yes | nats:2.6.1-alpine3.14 |


```
### [astra](https://cloud.netapp.com/astra)
| CRD Spec      | Details       | Optional | Default |
| ------------- | ------------- | -------- |--------|
| unregister    | Unregister the cluster with Astra | Yes | false |
| token         | Astra API token of a user with an Owner Role| Yes | |
| clusterName   | If and only if you are importing an AKS cluster that is marked in AKS as private, and not providing the K8S Config file to Astra, then you need the Name of the private AKS cluster.  Otherwise do not have this attribute, remove the line | Yes | empty|
| accountId     | Astra account ID | No | |
| acceptEULA    | End User License Agreement | No | no |

