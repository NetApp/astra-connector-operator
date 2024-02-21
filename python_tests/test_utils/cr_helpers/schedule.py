from kubernetes.client import ApiException

from python_tests import constants
from python_tests import defaults
from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper


class ScheduleHelper:
    created_schedules: list[dict] = []
    plural_name = "schedules"
    group = "astra.netapp.io"
    version = "v1"

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def gen_cr(self, name, app_name, app_vault_name, replicate: bool = False, interval: int = 5,
               frequency: str = constants.Frequency.MINUTELY.value, enabled=True):
        cr_def = {
            "apiVersion": f"{self.group}/{self.version}",
            "kind": "Schedule",
            "metadata": {
                "name": name,
            },
            "spec": {
                "applicationRef": app_name,
                "appVaultRef": app_vault_name,
                "recurrenceRule": f"DTSTART:20230928T110000Z\nRRULE:FREQ={frequency};INTERVAL={interval}",
                "enabled": enabled,
                "backupRetention": "0",
                "snapshotRetention": "1",
                "replicate": replicate,
                "granularity": "custom"
            }
        }
        return cr_def

    def apply_cr(self, cr_name, app_name, app_vault_name, replicate, interval: int = 5,
                 frequency: str = constants.Frequency.MINUTELY.value, enabled=True,
                 namespace=defaults.CONNECTOR_NAMESPACE) -> dict:
        cr_def = self.gen_cr(name=cr_name,
                             app_name=app_name,
                             app_vault_name=app_vault_name,
                             replicate=replicate,
                             interval=interval,
                             frequency=frequency,
                             enabled=enabled)
        cr_response = self.k8s_helper.apply_cr(cr_name, namespace, cr_def, self.plural_name)
        self.created_schedules.append(cr_response)
        return cr_response

    def get_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE):
        return self.k8s_helper.get_cr(name, namespace, self.group, self.version, self.plural_name)

    def delete_cr(self, name, namespace=defaults.CONNECTOR_NAMESPACE):
        self.k8s_helper.delete_cr(namespace, name, self.group, self.version, self.plural_name)

    def cleanup(self):
        for schedule in self.created_schedules:
            try:
                name = schedule.get('metadata', {}).get('name', '')
                namespace = schedule.get('metadata', {}).get('namespace', '')
                self.delete_cr(name=name, namespace=namespace)
            except ApiException as e:
                # Don"t fail if the CR has already been deleted
                if e.status != 404:
                    logger.warn(f"encountered error cleaning up schedule CRs: {e}")
