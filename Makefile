MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR := $(shell echo "$(MAKEFILE_PATH)" | sed 's,/Makefile,,' )

# Paths
SCRIPTS_DIR := $(MAKEFILE_DIR)/scripts
BUILD_DIR := $(MAKEFILE_DIR)/build
OUTPUT_IMAGE_TAR_DIR := $(BUILD_DIR)/images
INSTALL_DIR := $(MAKEFILE_DIR)/installer
OUTPUT_INSTALL_EXE_DIR := $(BUILD_DIR)/install-exe

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 3.0.1

OS ?= linux
ARCH ?= amd64
PLATFORM ?= ${OS}/${ARCH}

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# netapp.io/operator-bundle:$VERSION and netapp.io/operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= netapp/astra-connector-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.22

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	cd details/operator-sdk && $(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	cd details/operator-sdk && $(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

test: manifests generate fmt vet envtest ## Run tests.
	# exclude mocks and script folder
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) --arch=amd64 use $(ENVTEST_K8S_VERSION) -p path)" PROJECT_ROOT=$(PWD) go test $(shell go list ./... | grep -v /mocks | grep -v /scripts) -coverprofile coverage.out
	cat coverage.out | grep -v "zz_generated.deepcopy.go" > cover.out

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

docker-build: ## Build docker image with the manager.
	docker build --platform=$(PLATFORM)  -t ${IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	docker buildx build --push --platform=$(PLATFORM) --tag $(IMG) .

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	cd details/operator-sdk && $(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	cd details/operator-sdk && $(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd details/operator-sdk/config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	cd details/operator-sdk && $(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	cd details/operator-sdk && $(KUSTOMIZE) build config/default | kubectl delete -f -


CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.3)

ENVTEST = $(shell pwd)/bin/setup-envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

MOCKERY = $(shell pwd)/bin/mockery
install-mockery: ## Download mockery locally if necessary: https://github.com/vektra/mockery
	$(call go-get-tool,$(MOCKERY),github.com/vektra/mockery/v2@v2.19.0)

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
install-golangci-lint: ## Download golangci-lint locally if necessary: https://github.com/golangci/golangci-lint
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2)

# go-get-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
mkdir -p $(PROJECT_DIR)/bin ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2) into $(1)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) && echo "Downloaded $(1)" || echo "error downloading $(1)";\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd details/operator-sdk/config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	cd details/operator-sdk && $(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

image-tar:
	rm -rf ${OUTPUT_IMAGE_TAR_DIR}
	mkdir -p ${OUTPUT_IMAGE_TAR_DIR}
	$(SCRIPTS_DIR)/create-image-tar.sh ${OUTPUT_IMAGE_TAR_DIR}/astra-connector-operator-images.tar

# Creates release containing versioned YAMLs
.PHONY: release
release: kustomize
	rm -rf build
	mkdir build
	cd details/operator-sdk/config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	cd details/operator-sdk && $(KUSTOMIZE) build config/default > $(BUILD_DIR)/only_astraconnector_operator.yaml
	cat $(MAKEFILE_DIR)/unified-installer/neptune.yaml > $(BUILD_DIR)/astraconnector_operator.yaml
	echo "---" >> $(BUILD_DIR)/astraconnector_operator.yaml
	cat $(BUILD_DIR)/only_astraconnector_operator.yaml >> $(BUILD_DIR)/astraconnector_operator.yaml
	cp $(MAKEFILE_DIR)/details/operator-sdk/config/samples/astra_v1_astraconnector.yaml $(BUILD_DIR)/astra_v1_astraconnector.yaml

.PHONY: generate-mocks
generate-mocks: install-mockery
	./bin/mockery --all

cov_dir:
	@mkdir -p ${COV_DIR}
	@chmod ${DIRPERMS} ${COV_DIR}

.PHONY: l lint
l:
lint: fmt lint-go

lint-go: fmt install-golangci-lint
	$(GOLANGCI_LINT) cache status
	$(GOLANGCI_LINT) run --timeout 20m --verbose --print-resources-usage
