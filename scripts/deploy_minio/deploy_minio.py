from kubernetes import client, config
import argparse
import time
import base64


def create_namespace_if_not_exists(namespace):
    # Try to get the namespace
    try:
        v1.read_namespace(name=namespace)
    except client.exceptions.ApiException as e:
        # If the namespace does not exist, create it
        if e.status == 404:
            namespace_body = client.V1Namespace(metadata=client.V1ObjectMeta(name=namespace))
            v1.create_namespace(body=namespace_body)
        else:
            raise


def get_minio_pod_def(pvc_name, namespace):
    return {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
            "name": "minio",
            "namespace": namespace,
            "labels": {
                "app": "minio",
            },
        },
        "spec": {
            "containers": [
                {
                    "name": "minio",
                    "image": "quay.io/minio/minio:latest",
                    "command": ["/bin/bash", "-c"],
                    "args": ["minio server /data --console-address :9090"],
                    "volumeMounts": [
                        {
                            "mountPath": "/data",
                            "name": "minio-data",
                        }
                    ],
                }
            ],
            "volumes": [
                {
                    "name": "minio-data",
                    "persistentVolumeClaim": {
                        "claimName": pvc_name,
                    },
                }
            ],
            "env": [
                {
                    "name": "MINIO_ACCESS_KEY",
                    "valueFrom": {
                        "secretKeyRef": {
                            "name": "minio-secret",
                            "key": "access_key"
                        }
                    }
                },
                {
                    "name": "MINIO_SECRET_KEY",
                    "valueFrom": {
                        "secretKeyRef": {
                            "name": "minio-secret",
                            "key": "secret_key"
                        }
                    }
                }
            ],
        },
    }


def get_minio_service_def(namespace, node_port):
    return {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {
            "name": "minio",
            "namespace": namespace,
        },
        "spec": {
            "type": "NodePort",
            "selector": {
                "app": "minio",
            },
            "ports": [
                {
                    "protocol": "TCP",
                    "port": 9000,
                    "targetPort": 9000,
                    "nodePort": node_port,
                }
            ],
        },
    }


def get_minio_ui_service_def(namespace, node_port):
    return {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {
            "name": "minio-console-service",
            "namespace": namespace,
        },
        "spec": {
            "type": "NodePort",
            "selector": {
                "app": "minio",
            },
            "ports": [
                {
                    "protocol": "TCP",
                    "port": 9090,
                    "targetPort": 9090,
                    "nodePort": node_port,
                }
            ],
        },
    }


def get_create_minio_pvc_def(pvc_name, namespace):
    return {
        "apiVersion": "v1",
        "kind": "PersistentVolumeClaim",
        "metadata": {
            "name": pvc_name,
            "namespace": namespace,
        },
        "spec": {
            "accessModes": ["ReadWriteOnce"],
            "resources": {
                "requests": {
                    "storage": "1Gi",
                },
            },
        },
    }


def get_minio_secret_def(access_key, secret_key):
    access_encoded = base64.b64encode(access_key.encode()).decode()
    secret_encoded = base64.b64encode(secret_key.encode()).decode()
    return {
        "apiVersion": "v1",
        "kind": "Secret",
        "metadata": {
            "name": "minio-secret"
        },
        "type": "Opaque",
        "data": {
            "access_key": access_encoded,
            "secret_key": secret_encoded
        }
    }


# ------
# Main #
# ------


# Required command line arguments
parser = argparse.ArgumentParser(description='Create a MinIO Pod in Kubernetes.')

# Optional command line arguments
parser.add_argument('--access_key', type=str, default="minioadmin", help='Access key for MinIO.')
parser.add_argument('--secret_key', type=str, default="minioadmin", help='Secret key for MinIO.')
parser.add_argument('--node_port', type=int, default="30001", help='Port to expose MinIO on.')
parser.add_argument('--node_port_ui', type=int, default="30002", help='Port to expose MinIO UI on.')
parser.add_argument('--kubeconfig', type=str, default='~/.kube/config', help='Path to the kubeconfig file.')
parser.add_argument('--namespace', type=str, default='minio', help='Namespace to create the MinIO Pod in.')
parser.add_argument('--timeout', type=int, default=120, help='MinIO deploy timeout in seconds.')

# Parse cmd line args
args = parser.parse_args()

print(f"Loading kubeconfig from {args.kubeconfig}")
config.load_kube_config(config_file=args.kubeconfig)

# Create the API client
print("Creating K8s API client")
v1 = client.CoreV1Api()

# Create the namespace if it does not exist
print(f"Creating namespace {args.namespace}")
create_namespace_if_not_exists(args.namespace)

# # Create the PVC
pvc_name = "minio-pvc"
print(f"Creating PVC {pvc_name}")
v1.create_namespaced_persistent_volume_claim(
    namespace=args.namespace,
    body=get_create_minio_pvc_def(pvc_name, args.namespace),
)

# Create the Secret
print(f"Creating secret")
secret_def = get_minio_secret_def(args.access_key, args.secret_key)
v1.create_namespaced_secret(
    namespace=args.namespace,
    body=secret_def,
)

# Create the Service
minio_service = get_minio_service_def(namespace=args.namespace, node_port=args.node_port)
print(f"Creating k8s service")
v1.create_namespaced_service(
    namespace=args.namespace,
    body=minio_service,
)

# Create another service for UI
minio_ui_service = get_minio_ui_service_def(namespace=args.namespace, node_port=args.node_port_ui)
v1.create_namespaced_service(
    namespace=args.namespace,
    body=minio_ui_service,
)

# Define the MinIO Pod
minio_pod_def = get_minio_pod_def(pvc_name ,namespace=args.namespace)
print("Creating MinIO Pod")

# Create the MinIO Pod
v1.create_namespaced_pod(
    namespace=args.namespace,
    body=minio_pod_def,
)

# Wait for the Pod to start
pod_name = "minio"
time_expire = time.time() + args.timeout
running = False
while True and time.time() < time_expire:
    pod = v1.read_namespaced_pod(name=pod_name, namespace=args.namespace)
    if pod.status.phase == "Running":
        print(f"Pod {pod_name} is running.")
        running = True
        break
    else:
        print(f"Pod {pod_name} is not running yet. Waiting...")
        time.sleep(5)  # Wait before checking again
if not running:
    raise TimeoutError("timed out waiting for pod(s) to start")
