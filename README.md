# Astra Agent Operator 

Astra Agent Operator deploys and registers a private azure cluster with [NetApp Astra](https://cloud.netapp.com/astra)

### To deploy the operator
Create the namespace for the operator
```
kubectl create ns astra-agent-operator
```
Apply the astraagent_operator.yaml file to the operator namespace
```
kubectl apply -f astraagent_operator.yaml -n astra-agent-operator
```
### Install the private cluster components
Create the namespace for the private cluster components
```
kubectl create ns astra-agent
```
Apply the AstraAgent CRD

Update the CRD with the right values. Refer to the table below for explanations of the CRD spec
```
kubectl apply -f config/samples/astraagent_v1.yaml -n astra-agent
```

Apply the AstraAgent CRD
```
kubectl apply -f config/samples/astraagent_v1.yaml -n astra-agent
```
### Uninstall the private cluster components
- Unmanage the cluster from the Astra UI
- Remove the AstraAgent CRD

NOTE: Removing the CRD will also attempt to unregister the cluster with Astra
```
kubectl delete -f config/samples/astraagent_v1.yaml -n astra-agent
```
### Uninstall the operator
```
kubectl delete -f astraagent_operator.yaml -n astra-agent-operator
```
## CRD
Sample CRD
```
apiVersion: netapp.astraagent.com/v1
kind: AstraAgent
metadata:
  name: astra-agent
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

| CRD Spec          | Details       | Optional | Default |
| ----------------- | ------------- |--------- | --------|
| image   | natssync-client image | Yes | natssync-client:0.9.202202161623 |
| cloud-bridge-url  | Astra URL  | Yes | https://integration.astra.netapp.io |
| skipTLSValidation | Skip TLS Validation| Yes| false |
| hostalias | Use a custom IP for the cloud bridge hostname| Yes | false |
| hostaliasIP | IP to use for host alias | Yes if hostalias is false | |

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

Example:
```
spec:
    ...
    nats:
        size: 3
        image: nats:2.6.1-alpine3.14
```

### [httpproxy-client](https://github.com/theotw/natssync)
| CRD Spec | Details       | Optional | Default |
| ---------| ------------- |--------- | --------|
| image    | httpproxy-client image | Yes | httpproxylet:0.9.202202161623 |

Example:
```
spec:
    ...
    httpproxy-client:
        image: theotw/httpproxylet:0.9.202201132025
```
### [echo-client](https://github.com/theotw/natssync)
| CRD Spec | Details       | Optional | Default |
| ---------| ------------- |--------- | --------|
| size     | Replica count for the echo-client deployment | Yes | 1 |
| image    | echo-client image | Yes | echo-proxylet:0.9.202202161623 |

Example:
```
spec:
    ...
    echo-client:
        size: 1
        image: theotw/echo-proxylet:0.9.202201132025

```
### [astra](https://cloud.netapp.com/astra)
| CRD Spec      | Details       | Optional | Default |
| ------------- | ------------- | -------- |--------|
| unregister    | Unregister the cluster with Astra | Yes | false |
| token         | Astra API token of a user with an Owner Role| Yes | |
| clusterName   | Name of the private AKS cluster | No | |
| accountId     | Astra account ID | No | |
| acceptEULA    | End User License Agreement | No | no |

