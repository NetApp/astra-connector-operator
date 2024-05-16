# (BETA) Astra Cluster Install Script
The cluster install script will manage an arch3.0 cluster in one go.
It will also install/upgrade Trident and enable ACP if required/desired.

## Files
- `install.sh`: this is the main script. It will install the Astra Connector and manage
install/upgrade Trident as needed.
- `kustomization.yaml`: this is a temporary kustomization pulled in by the install
  script to install the neptune CRDs. It will be removed once the operator is updated to
  install them via Helm.

## Getting Started (UI)
If you got here from the Astra UI, the first thing to note is that the instructions you
were provided are not dev-ready. To run the script, you will need to set some environment
variables on top of what's already set in the instructions provided.

1. If you're using a development cluster, you'll likely need to set:
```
SKIP_TLS_VALIDATION=true
```
2. Depending on which registries you have access to, you will need to overwrite the
default Connector/Neptune/Trident-ACP registries:
```
ASTRA_IMAGE_REGISTRY=netappdownloads.jfrog.io
ASTRA_BASE_REPO=docker-astra-control-staging/arch30/neptune
TRIDENT_ACP_IMAGE_REGISTRY=netappdownloads.jfrog.io
TRIDENT_ACP_IMAGE_REPO=oss-docker-trident-staging/astra/trident-acp
```
3. If you want to use specific image tags, you can do so as well:
```
CONNECTOR_OPERATOR_IMAGE_TAG=202405141943-main
CONNECTOR_IMAGE_TAG=25d40e8
NEPTUNE_IMAGE_TAG=aaa9b9c
TRIDENT_ACP_IMAGE_TAG=24.02
```
4. Finally, you will need to create an image pull secret for the registries configured in step 2
and set the appropriate environment variable:
```
IMAGE_PULL_SECRET=my-pull-secret
```
5. And because this is a lot to do in one command, you can put all of environment variables
in a file (see [install-example-config.env](install-example-config.env)) and replace them with:
```
CONFIG_FILE=/path/to/my-config.env
```

## Getting Started (Command Line)
The first step is to make a copy of the `install-example-config.env` before anything.
All **.env** files other than the example are in the .gitignore, so you'll be able to
easily modify and carry around your config without checking anything in.

Once that's done, simply fill out the fields in a way that makes sense for your current
environment, and then run the script:
```shell
CONFIG_FILE=my-config.env ./install.sh
```
Note: DRY_RUN is set to true by default. Once you're ready to test the script for real,
just set DRY_RUN=false.

## Development Style Guide
- Global variables are in full upper case, e.g. `MY_VARIABLE`
- Stateful globals are prefixed with one underscore, e.g. `_MY_VARIABLE`
- Constants globals are prefixed with two underscores, e.g. `__MY_VARIABLE`
- Functions containing the higher-level business logic are prefixed with `step_`, e.g.
  `step_generate_some_yaml`

For everything else, simply try to be consistent with what you see and follow your common sense!
