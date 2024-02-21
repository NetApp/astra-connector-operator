from python_tests.test_utils.cr_helpers.app_vault import AppVaultHelper
from python_tests.test_utils.cr_helpers.appmirror import AppMirrorHelper
from python_tests.test_utils.cr_helpers.schedule import ScheduleHelper
from python_tests.test_utils.cr_helpers.snapshot import SnapshotHelper
from python_tests.test_utils.cr_helpers.application import ApplicationHelper
from python_tests.test_utils.app_installer import AppInstaller
from python_tests.test_utils.k8s_helper import K8sHelper
from python_tests.test_utils.cr_helpers.backup import BackupHelper


class Cluster:

    def __init__(self, kubeconfig_path, default_bucket):
        self.k8s_helper = K8sHelper(kubeconfig_path)
        self.app_installer = AppInstaller(self.k8s_helper)
        self.default_test_bucket = default_bucket

        # --- CR Helpers ---
        self.app_vault = AppVaultHelper(self.k8s_helper)
        self.snapshot_helper = SnapshotHelper(self.k8s_helper)
        self.application_helper = ApplicationHelper(self.k8s_helper)
        self.backup_helper = BackupHelper(self.k8s_helper)
        self.schedule_helper = ScheduleHelper(self.k8s_helper)
        self.appmirror_helper = AppMirrorHelper(self.k8s_helper)

    def cleanup(self):
        self.snapshot_helper.cleanup()
        self.backup_helper.cleanup()
        self.application_helper.cleanup()
        self.app_vault.cleanup()
        self.k8s_helper.cleanup()
        self.app_installer.cleanup()

