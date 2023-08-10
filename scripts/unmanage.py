import argparse
from typing import Tuple
import requests
from requests.models import Response


def main(cloud_bridge_host, cloud_id, cluster_id):
    print(f"Using Cloud Bridge Host: {cloud_bridge_host}")
    print(f"Using Cloud ID: {cloud_id}")
    print(f"Using Cluster ID: {cluster_id}")

    remove_cluster(cloud_bridge_host, cloud_id, cluster_id)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Unmanage Astra Conncetor")
    parser.add_argument(
        "--cloudBridgeHost",
        default="astra.netapp.io",
        help="Host for the Cloud Bridge (default: astra.netapp.io)",
    )
    parser.add_argument(
        "--cloudId",
        help="ID of Cloud that is being used for unregistration",
    )
    parser.add_argument(
        "--clusterId",
        help="ID of north side cluster that is being used for unregistered from",
    )

    args = parser.parse_args()
    main(args.cloudBridgeUrl, args.cloudId, args.clusterId)


def remove_cluster(self, astra_host: str, cloud_id: str, cluster_id: str) -> Tuple[Response, Exception]:
    url = f"{astra_host}/accounts/{self.astra_installer['spec']['connectorSpec']['astra']['accountID']}/topology/v1" \
          f"/clouds/{cloud_id}/clusters/{cluster_id}"
    self.log.info("Removing Cluster", extra={"clusterID": cluster_id})
    headers = {"Authorization": f"Bearer {self.astra_installer['spec']['connectorSpec']['astra']['token']}"}

    try:
        response = requests.delete(url, headers=headers)
        response.raise_for_status()
    except requests.exceptions.RequestException as err:
        return None, Exception(f"Error on request delete cluster with id: {cluster_id}. {err}")

    if response.status_code != 204:
        return None, Exception(f"Remove cluster failed with statusCode: {response.status_code}")

    return response, None
