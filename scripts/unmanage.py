import argparse
import logging
import json
import requests


class UnmanageUtil:
    def __init__(self, log, astra_url, account_id, token, cluster_name):
        self.log = log
        self.astra_url = astra_url
        self.account_id = account_id
        self.token = token
        self.cluster_name = cluster_name

    def remove_cluster(self, cloud_id: str, cluster_id: str) -> None:
        try:
            url = f"{self.astra_url}/accounts/{self.account_id}/topology/v1" \
                  f"/clouds/{cloud_id}/clusters/{cluster_id}"
            self.log.info("Removing Cluster", extra={"clusterID": cluster_id})
            headers = {"Authorization": f"Bearer {self.token}"}

            response = requests.delete(url, headers=headers, verify=False)
            response.raise_for_status()

        except (requests.exceptions.RequestException, json.JSONDecodeError, KeyError) as err:
            self.log.error(f"Error on request delete cluster with id: {cluster_id}. {err}")
            raise err

    def get_cloud_id(self) -> str:
        try:
            response = self.list_clouds()
            resp = response.json()
            cloud_id = next((item["id"] for item in resp["items"] if item["cloudType"] == "private"), "")
            return cloud_id
        except (requests.exceptions.RequestException, json.JSONDecodeError, KeyError) as err:
            self.log.error(f"Error listing clouds: {err}")
            raise err

    def list_clouds(self) -> requests.Response:
        url = f"{self.astra_url}/accounts/{self.account_id}/topology/v1/clouds"

        self.log.info("Getting clouds")
        return do_get_request(url, self.token)

    def get_clusters(self, cloud_id: str) -> dict:
        url = f"{self.astra_url}/accounts/{self.account_id}/topology/v1/clouds/{cloud_id}/clusters"

        self.log.info("Getting clusters")
        resp = do_get_request(url, self.token)
        return resp.json()

    def process_clusters(self, clusters_resp: dict) -> str:
        cluster_info = {}

        for value in clusters_resp["items"]:
            if value["name"] == self.cluster_name:
                self.log.info(f"Found the required cluster info: ClusterId={value['id']}, Name={value['name']}, "
                              f"ManagedState={value['managedState']}")
                cluster_info = {
                    "id": value["id"],
                    "managedState": value["managedState"],
                    "name": value["name"]
                }

        if not cluster_info:
            raise Exception("Required cluster not found")

        return cluster_info["id"]


def do_get_request(url, token) -> requests.Response:
    headers = {
        "Authorization": f"Bearer {token}"
    }

    response = requests.get(url, headers=headers, verify=False)
    response.raise_for_status()
    return response


def setup_logging():
    log_format = "%(asctime)s - %(levelname)s - %(message)s"
    logging.basicConfig(level=logging.INFO, format=log_format)
    return logging.getLogger()


def main(astra_url, account_id, token, cluster_name):
    print(f"Using Cloud Bridge Host: {astra_url}")
    print(f"Using Account ID: {account_id}")

    log = setup_logging()
    util = UnmanageUtil(log, astra_url, account_id, token, cluster_name)

    cloud_id = util.get_cloud_id()
    clusters_resp = util.get_clusters(cloud_id)
    cluster_id = util.process_clusters(clusters_resp)

    util.remove_cluster(cloud_id, cluster_id)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Unmanage Astra Conncetor")
    parser.add_argument(
        "--astraUrl",
        default="https://astra.netapp.io",
        help="Host for the Cloud Bridge (default: https://astra.netapp.io)",
    )
    parser.add_argument(
        "--accountId",
        help="ID of account that is being used for unregistration",
    )
    parser.add_argument(
        "--authToken",
        help="Token being used for authentication",
    )
    parser.add_argument(
        "--clusterName",
        help="Name of cluster being used for unregistration",
    )

    args = parser.parse_args()
    main(args.astraUrl, args.accountId, args.authToken, args.clusterName)
