This folder is where the Astra cluster install script will live.
- `kustomization.yaml`: this is a temporary kustomization pulled in by the install 
script to install the neptune CRDs. It will be removed once the operator is updated to
install them via Helm.
