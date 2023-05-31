#!/bin/bash

# Set variables
RESOURCE="controllerconfig.yaml"
OPERATOR_DEPLOY="astraconnector_operator.yaml"
NAMESPACE="astra-connector-operator"

# Create the namespace if it doesn't exist
kubectl get namespace "${NAMESPACE}" &> /dev/null || kubectl create namespace "${NAMESPACE}"

# Deploy the operator
kubectl apply -f "${OPERATOR_DEPLOY}" -n "${NAMESPACE}"

# Wait for the operator to be running
while [[ $(kubectl get pods -n "${NAMESPACE}" -l control-plane=controller-manager -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do
  echo "Waiting for the operator to be ready..."
  sleep 5
done

# Create an instance of the custom resource
kubectl apply -f "${RESOURCE}" -n "${NAMESPACE}"

echo "Operator deployed and custom resource created successfully."