# Astra Cluster Install Script
The cluster install script will manage an arch3.0 cluster in one go.
It will also install/upgrade Trident and enable ACP if required/desired.

It will be moved to the Astra Connector Operator repo once it is
in a more polished state.

## Files
- `install.sh`: this is the main script. It will install the Astra Connector and manage
install/upgrade Trident as needed.
- `kustomization.yaml`: this is a temporary kustomization pulled in by the install
  script to install the neptune CRDs. It will be removed once the operator is updated to
  install them via Helm.

## Getting Started
The first step is to make a copy of the `cluster-install-example.env` before anything.
All **.env** files other than the example are in the .gitignore, so you'll be able to
easily modify and carry around your config without checking anything in.

Once that's done, simply fill out the fields in a way that makes sense for your current
environment, and then run the script:
```shell
CONFIG_FILE=my-config.env ./install.sh
```
Note: DRY_RUN is set to true by default. Once you're ready to test the script for real,
just set DRY_RUN=false.

## Style Guide
- Global variables are in full upper case, e.g. `MY_VARIABLE`
- Stateful globals are prefixed with one underscore, e.g. `_MY_VARIABLE`
- Constants globals are prefixed with two underscores, e.g. `__MY_VARIABLE`
- Functions containing the higher-level business logic are prefixed with `step_`, e.g.
  `step_generate_some_yaml`

For everything else, simply try to be consistent with what you see and follow your common sense!
