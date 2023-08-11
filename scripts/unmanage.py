import argparse
import logging
import json
import requests


class UnmanageUtil:
    def __init__(self, log):
        self.log = log

    def remove_cluster(self, astra_host: str, account_id: str, cloud_id: str, cluster_id: str, token: str) -> None:
        try:
            url = f"{astra_host}/accounts/{account_id}/topology/v1" \
                  f"/clouds/{cloud_id}/clusters/{cluster_id}"
            self.log.info("Removing Cluster", extra={"clusterID": cluster_id})
            headers = {"Authorization": f"Bearer {token}"}

            response = requests.delete(url, headers=headers)
            response.raise_for_status()

        except (requests.exceptions.RequestException, json.JSONDecodeError, KeyError) as err:
            self.log.error(f"Error on request delete cluster with id: {cluster_id}. {err}")
            raise err

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


def setup_logging():
    log_format = "%(asctime)s - %(levelname)s - %(message)s"
    logging.basicConfig(level=logging.INFO, format=log_format)
    return logging.getLogger()


def main(astra_host, account_id, token):
    print(f"Using Cloud Bridge Host: {astra_host}")
    print(f"Using Account ID: {account_id}")

    log = setup_logging()
    util = UnmanageUtil(log)

    cloud_id = util.get_cloud_id(astra_host, account_id, token)

    remove_cluster(astra_host, account_id, cloud_id, cluster_id, token)


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
