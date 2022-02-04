# Astra Agent Operator 

Astra Agent Operator deploys and registers a private azure cluster with [NetApp Astra](https://cloud.netapp.com/astra)

### To deploy the operator
Create the namespace for the operator
```
kubectl create ns astra-agent-operator
```
Apply the operator.yaml file to the operator namespace
```
kubectl apply -f operator.yaml -n astra-agent-operator
```
### Install the private cluster components
Create the namespace for the private cluster components
```
kubectl create ns astra-agent
```
Apply the AstraAgent CRD

Update the CRD with the right values. Refer to the table below for explanations of the CRD spec
```
kubectl apply -f config/samples/cache_v1_astraagent.yaml -n astra-agent
```
### Unregister the private cluster with Astra
- Unmanage the cluster from the Astra UI
- Change the value of `register` to false in the CRD
```
spec:
  ......
  astra:
    register: false
```
Apply the AstraAgent CRD
```
kubectl apply -f config/samples/cache_v1_astraagent.yaml -n astra-agent
```
### Uninstall the private cluster components
- Unmanage the cluster from the Astra UI
- Remove the AstraAgent CRD

NOTE: Removing the CRD will also attempt to unregister the cluster with Astra
```
kubectl delete -f config/samples/cache_v1_astraagent.yaml -n astra-agent
```
### Uninstall the operator
```
kubectl delete -f operator.yaml -n astra-agent-operator
```
## CRD
Sample CRD
```
apiVersion: cache.astraagent.com/v1
kind: AstraAgent
metadata:
  name: astra-agent
spec:
  namespace: astra-agent
  natssync-client:
    name: natssync-client
    size: 1
    image: theotw/natssync-client:0.9.202201132025
    cloud-bridge-url: https://integration.astra.netapp.io
    port: 8080
    protocol: TCP
    keystoreUrl: configmap:///configmap-data
  nats:
    name: nats
    cluster-service-name: nats-cluster
    configMapName: nats-configmap
    serviceaccountname: nats-serviceaccount
    volumename: nats-configmap-volume
    size: 2
    image: nats:2.6.1-alpine3.14
    client-port: 4222
    cluster-port: 6222
    monitor-port: 8222
    metrics-port: 7777
    gateways-port: 7522
  httpproxy-client:
    name: httpproxy-client
    size: 1
    image: theotw/httpproxylet:0.9.202201132025
  echo-client:
    name: echo-client
    size: 1
    image: theotw/echo-proxylet:0.9.202201132025
  configMap:
    name: natssync-client-configmap
    rolename: natssync-client-configmap-role
    rolebindingname: natssync-client-configmap-rolebinding
    serviceaccountname: natssync-client-configmap-serviceaccount
    volumename: natssync-client-configmap-volume
  astra:
    register: false
    token: <AstraApiToken>
    clusterName: ipsprintdemo
    accountId: 5831cf97-c52e-4185-ab83-db0fdd061c0e
    cloudType: Azure
```

## CRD details
In the CRD, all the fields in the spec section have to be updated to the correct value

### Namespace
namespace is where the CRD and hence all the private cluster components get installed
Example:
```
namespace: astra-agent
```
### [natssync-client](https://github.com/theotw/natssync)
Natssync-Client talks to the Natssync-Server on Astra and ensures communication between Astra and the private AKS cluster

The natssync-client CRD is a map of key/value pairs

| CRD Spec          | Details       |
| ----------------- | ------------- |
| image             | natssync-client image |
| cloud-bridge-url  | Astra URL  |

### [nats](https://nats.io/)
| CRD Spec             | Details       |
| ---------------------| ------------- |
| size                 | Replica count for the nats statefulset |
| image                | nats image |

### [httpproxy-client](https://github.com/theotw/natssync)
| CRD Spec | Details       |
| ---------| ------------- |
| image    | httpproxy-client image |

### [echo-client](https://github.com/theotw/natssync)
| CRD Spec | Details       |
| ---------| ------------- |
| size     | Replica count for the echo-client deployment |
| image    | echo-client image |

### [astra](https://cloud.netapp.com/astra)
| CRD Spec      | Details       |
| ------------- | ------------- |
| register      | (Un)Register the cluster with Astra |
| token         | Astra API token of a user with an Owner Role|
| clusterName   | Name of the private AKS cluster |
| accountId     | Astra account ID |
| cloudType     | Cloud Type of the private cluster - Azure |

