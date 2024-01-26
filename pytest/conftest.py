import pytest


def pytest_addoption(parser):
    parser.addoption(
        "--kubeconfig", action="store", default="~/.kube/config", help="Path to the kubeconfig file"
    )


@pytest.fixture(scope="session")
def kubeconfig(request):
    return request.config.getoption("--kubeconfig")

