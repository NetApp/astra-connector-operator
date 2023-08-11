import argparse
import json
import requests


def main(astra_host, account_id, auth_token):
    print(f"Using Cloud Bridge Host: {astra_host}")
    print(f"Using Account ID: {account_id}")

    remove_cluster(astra_host, account_id, auth_token)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Unmanage Astra Conncetor")
    parser.add_argument(
        "--astraHost",
        default="astra.netapp.io",
        help="Host for the Cloud Bridge (default: astra.netapp.io)",
    )
    parser.add_argument(
        "--accountId",
        help="ID of account that is being used for unregistration",
    )
    parser.add_argument(
        "--authToken",
        help="Token being used for authentication",
    )

    args = parser.parse_args()
    main(args.astraHost, args.accountId, args.authToken)


def remove_cluster(self, astra_host: str, account_id: str, token: str) -> None:
    cloud_id = get_cloud_id(astra_host, token)

    url = f"{astra_host}/accounts/{account_id}/topology/v1" \
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


def get_cloud_id(self, astra_host: str, account_id: str, token: str) -> str:
    try:
        response = self.list_clouds(astra_host, account_id, token)
        resp = response.json()
        cloud_id = next((item["id"] for item in resp["items"] if item["cloudType"] == "private"), "")
        return cloud_id
    except (requests.exceptions.RequestException, json.JSONDecodeError, KeyError) as err:
        self.log.error(f"Error listing clouds: {err}")
        raise err


def list_clouds(self, astra_host: str, account_id: str, token: str) -> requests.Response:
    url = f"{astra_host}/accounts/{account_id}/topology/v1/clouds"

    self.log.info("Getting clouds")
    headers = {
        "Authorization": f"Bearer {token}"
    }

    response = requests.get(url, headers=headers)
    response.raise_for_status()
    return response
