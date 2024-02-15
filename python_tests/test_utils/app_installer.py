import os
import subprocess
from python_tests.log import logger
from python_tests.test_utils.k8s_helper import K8sHelper
import python_tests.test_utils.random as random


# App holds useful information about created apps tests may care about
class App:

    def __init__(self, name, namespace):
        self.name = name
        self.namespace = namespace


class AppInstaller:

    def __init__(self, k8s_helper: K8sHelper):
        self.k8s_helper = k8s_helper

    def install_mariadb(self, namespace) -> App:

        helm_release_name = f"test-maria-{random.get_short_uuid()}"
        # The command to install MariaDB using Helm, uses the --wait option to wait for pods to start
        command = ["helm", "install", "test-maria", "bitnami/mariadb", "--set", "auth.rootPassword=password",
                   "--version", "9.3.14", "--wait", "--timeout 60", "-n", namespace]

        # Set KUBECONFIG env var
        env = os.environ.copy()
        env["KUBECONFIG"] = self.k8s_helper.kubeconfig

        # Execute the command
        logger.info(f"Installing mariadb with cmd '{' '.join(command)}'")
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

        # Wait for the command to complete
        stdout, stderr = process.communicate()

        # Check if the command was successful
        if process.returncode != 0:
            logger.info(f"Error installing MariaDB: {stderr.decode('utf-8')}")
        else:
            logger.info(f"MariaDB installed successfully: {stdout.decode('utf-8')}")

        return App(name=helm_release_name, namespace=namespace)
