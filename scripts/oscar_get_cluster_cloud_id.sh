#!/bin/bash

## Please fill this out
astra_url="https://dev-ol-astra-enterprise-team3-03-lb.rtp.openenglab.netapp.com"
account_id="bb22ca62-48ad-4697-a4d3-64bded53d849"
auth_token="gmiQt963pM9iRC2E-6a5EwuUzkT5w2iIk87_ca6x-DM="
cluster_name="bobby"

# Define the base URL and headers for convenience
base_url="${astra_url}/accounts/${account_id}/topology/v1/clouds"
accept_header="accept: application/json"
auth_header="Authorization: Bearer ${auth_token}"
cluster_name_unique=${cluster_name}-$(date +%m-%d-%H-%M%S)

# See if cloud exists
response=$(curl -s -S -k -X 'GET' \
  "${base_url}?include=id%2Cname%2Cstate&filter=name%20eq%20%27private%27" \
  -H "${accept_header}" \
  -H "${auth_header}")
# check see GET wored
if [ $? -ne 0 ]; then
  echo "Error: Failed to make GET request"
  exit 1
fi

# Extract the items array from the response
items=$(echo "${response}" | jq '.items')

# Check if the items array has exactly one item
if [ $(echo "${items}" | jq 'length') -eq 1 ]; then
  # If it does, extract the id and store it in cloudID
  cloudID=$(echo "${items}" | jq -r '.[0][0]')
else
  # If it doesn't, make a POST request to create the item
  post_response=$(curl -k -X 'POST' \
    "${base_url}" \
    -H "${accept_header}" \
    -H "${auth_header}" \
    -H 'Content-Type: application/astra-cloud+json' \
    -d '{
      "cloudType": "private",
      "name": "private",
      "type": "application/astra-cloud",
      "version": "1.1"
    }')

  # Extract the id from the response and store it in cloudID
  cloudID=$(echo "${post_response}" | jq -r '.id')
fi

# Now you can use $cloudID in the rest of your script
#echo "Cloud ID: ${cloudID}"



# Make the POST request using the cloudID
cluster_response=$(curl -s -S -k -X 'POST' \
  "${base_url}/${cloudID}/clusters" \
  -H 'accept: application/astra-cluster+json' \
  -H "${auth_header}" \
  -H 'Content-Type: application/astra-cluster+json' \
  -d '{
    "name": "'"${cluster_name_unique}"'",
    "type": "application/astra-cluster",
    "connectorInstall": "pending",
    "version": "1.6"
  }')

# Check if the curl command was successful
if [ $? -ne 0 ]; then
  echo "Error: Failed to make POST request"
  exit 1
fi

# Extract the id from the response and store it in clusterID
clusterID=$(echo "${cluster_response}" | jq -r '.id')
#echo "Cluster ID: ${clusterID}"

# Print the YAML template, replacing the placeholders with the variables
echo "apiVersion: astra.netapp.io/v1
kind: AstraConnector
metadata:
  name: astra-connector
  namespace: astra-connector
spec:
  astra:
    accountId: ${account_id}
    clusterId: ${clusterID}
    cloudId: ${cloudID}
    skipTLSValidation: true
    tokenRef: astra-api-token
  natsSyncClient:
    cloudBridgeURL: ${astra_url}
    # hostAliasIP: IP needed if cloudBridgeURL DNS is not routable
  imageRegistry:
    name: netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune
    secret: regcred"