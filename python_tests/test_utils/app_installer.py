import os
import subprocess

from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper


# App holds useful information about created apps tests may care about
class App:

    def __init__(self, name: str, namespace: str, k8s_helper: K8sHelper, storage_class: str = "default"):
        self.name = name
        self.namespace = namespace
        self.k8s_helper = k8s_helper
        self.storage_class = storage_class

    def install(self):
        pass  # todo enforce base child method

    def uninstall(self):
        pass  # todo enforce child class method


class MariaDb(App):

    def __init__(self, name: str, namespace: str, k8s_helper: K8sHelper, storage_class: str = "default"):
        super().__init__(name, namespace, k8s_helper, storage_class)

    def install(self):
        # Installs latest chart
        timeout = 90
        # The command to install MariaDB using Helm, uses the --wait option to wait for pods to start
        command = ["helm", "install", self.name, "oci://registry-1.docker.io/bitnamicharts/mariadb", "--set",
                   "auth.rootPassword=password", "--wait", "--timeout", f"{timeout}s", "-n", self.namespace]

        if self.storage_class != "default":
            command.extend(["--set", f"primary.persistence.storageClass=${self.storage_class}"])

        # Set KUBECONFIG env var
        env = os.environ.copy()
        env["KUBECONFIG"] = self.k8s_helper.kubeconfig

        # Create namespace
        self.k8s_helper.create_namespace(self.namespace)

        # Execute the command
        logger.info(f"Installing mariadb with cmd '{' '.join(command)}'")
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)

        # Wait for the command to complete
        stdout, stderr = process.communicate()

        # Check if the command was successful
        if process.returncode != 0:
            raise Exception(f"Error installing MariaDB: {stderr.decode('utf-8')}")
        else:
            logger.info(f"MariaDB installed successfully: {stdout.decode('utf-8')}")

    def uninstall(self):
        command = ["helm", "uninstall", self.name, "-n", self.namespace]
        # Execute the command
        logger.info(f"Installing mariadb with cmd '{' '.join(command)}'")
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

        # Wait for the command to complete
        stdout, stderr = process.communicate()

        # Check if the command was successful
        if process.returncode != 0:
            logger.info(f"Error uninstalling MariaDB: {stderr.decode('utf-8')}")
        else:
            logger.info(f"MariaDB uninstalled successfully: {stdout.decode('utf-8')}")

        self.k8s_helper.delete_namespace(self.namespace)


class AppInstaller:
    created_apps: list[App] = []

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def install_mariadb(self, name, namespace, storage_class="default") -> App:
        maria_db = MariaDb(name, namespace, self.k8s_helper, storage_class)
        maria_db.install()
        self.created_apps.append(maria_db)
        return maria_db

    def cleanup(self):
        for app in self.created_apps:
            app.uninstall()
