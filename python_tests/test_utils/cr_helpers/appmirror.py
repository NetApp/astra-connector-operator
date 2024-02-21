import time
from enum import Enum
from kubernetes.client import ApiException

import python_tests.constants as constants
import python_tests.defaults as defaults
from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper


class AppMirrorHelper:
    created_appmirrors: list[dict] = []
    plural_name = "appmirrorrelationships"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_cr(self, name, app_name, desired_state, src_app_vault_name, dest_app_vault_name, snap_path, sc_name,
               ns_mapping: [{}] = None, interval: int = 5, frequency: str = constants.Frequency.MINUTELY.value):
        cr_def = {
            "apiVersion": f"{self.group}/{self.version}",
            "kind": "AppMirrorRelationship",
            "metadata": {
                "name": name
            },
            "spec": {
                "applicationRef": app_name,
                "desiredState": desired_state,
                "sourceAppVaultRef": src_app_vault_name,
                "destinationAppVaultRef": dest_app_vault_name,
                "sourceSnapshotsPath": snap_path,
                "recurrenceRule": f"DTSTART:20230928T150000Z\nRRULE:FREQ={frequency};INTERVAL={interval}",
                "storageClassName": sc_name
            }
        }
        if ns_mapping:
            cr_def['spec']['namespaceMapping'] = ns_mapping
        return cr_def

    def apply_cr(self, cr_name, app_name, desired_state, src_app_vault_name, dest_app_vault_name, snap_path,
                 sc_name, ns_mapping: [{}] = None, interval: int = 5,
                 frequency: str = constants.Frequency.MINUTELY.value,
                 namespace=defaults.CONNECTOR_NAMESPACE) -> dict:
        cr_def = self.gen_cr(name=cr_name,
                             app_name=app_name,
                             desired_state=desired_state,
                             src_app_vault_name=src_app_vault_name,
                             dest_app_vault_name=dest_app_vault_name,
                             snap_path=snap_path,
                             sc_name=sc_name,
                             ns_mapping=ns_mapping,
                             interval=interval,
                             frequency=frequency)
        cr_response = self.k8s_helper.apply_cr(cr_name, namespace, cr_def, self.plural_name)
        self.created_appmirrors.append(cr_response)
        return cr_response

    def get_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def delete_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def update_cr(self, name, cr_def: dict, namespace=defaults.CONNECTOR_NAMESPACE):
        self.k8s_helper.update_cr(namespace, name, cr_def, self.plural_name)

    def cleanup(self):
        for app_mirror in self.created_appmirrors:
            try:
                name = app_mirror.get('metadata', {}).get('name', '')
                namespace = app_mirror.get('metadata', {}).get('namespace', '')
                self.delete_cr(name=name, namespace=namespace)
            except ApiException as e:
                # Don"t fail if the CR has already been deleted
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up appmirror CRs: {e}")

    def wait_for_state_with_timeout(self, cr_name: str, appmirror_state: str, timeout_sec: int,
                                    namespace: str = defaults.CONNECTOR_NAMESPACE):
        time_expire = time.time() + timeout_sec
        cr = None
        while time.time() < time_expire:
            cr = self.get_cr(cr_name, namespace)
            state = cr.get('status', {}).get("state", "")
            if state == appmirror_state:
                return
            time.sleep(3)

        raise TimeoutError(f"Timed out waiting for appmirror {cr_name} state '{appmirror_state}'\n{cr}")
