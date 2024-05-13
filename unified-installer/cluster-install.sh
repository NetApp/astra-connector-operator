#!/bin/bash

# -- Resources
# Design Page (detailed): https://confluence.ngage.netapp.com/pages/viewpage.action?pageId=805543984
# Deploy Trident Operator: https://docs.netapp.com/us-en/trident/trident-get-started/kubernetes-deploy-operator.html
# Upgrade Trident: https://docs.netapp.com/us-en/trident/trident-managing-k8s/upgrade-trident.html

# TODO ASTRACTL-32773: when upgrading an existing trident install that isn't in the same namespace as the given NAMESPACE,
# the upgrade will fail because of the namespace-transformer which will overwrite the existing trident
# install namespace on every resource (of particular note is the clusterrolebindings, which will break because
# it will be binding the cluster role to a service account in NAMESPACE instead of the existing trident namespace).
# Work-around: decline Trident upgrade, or use COMPONENTS=TRIDENT_ONLY if wanting to upgrade Trident
# TODO ASTRACTL-32773: do more testing with NAMESPACE="" once we a more stable kustomize structure in ACOP/Trident repos
# TODO ASTRACTL-32773: implement LABELS (env var is there but not used)
# TODO ASTRACTL-32772: SHOULD_UPGRADE_TRIDENT_IF_POSSIBLE var for automation (allowing upgrade to be skipped when DISABLE_PROMPTS=true)
# TODO ASTRACTL-32772: more in-depth CLOUD_ID and CLUSTER_ID check (duplicate cluster and such, see registration.go)
# TODO ASTRACTL-32138 (dependency): use _KUBERNETES_VERSION to choose the right bundle yaml (pre 1.25 or post 1.25)
# TODO: look into how the trident-autosupport tag is determined as it's preventing ACP from being enabled

#----------------------------------------------------------------------
#-- Private vars/constants
#----------------------------------------------------------------------

# ------------ STATEFUL VARIABLES ------------
_PROBLEMS=() # Any failed checks will be added to this array and the program will exit at specific checkpoints if not empty
_CONFIG_BUILDER=() # Contains the fully resolved config to be output at the end during a dry run
_PATCHES=() # Contains the k8s patches that will be applied alongside CRDs, CRs, etc.
_KUBERNETES_VERSION=""

_EXISTING_TORC_NAME="" # TORC is short for TridentOrchestrator (works with kubectl too)
_EXISTING_TRIDENT_NAMESPACE=""
_EXISTING_TRIDENT_IMAGE=""
_EXISTING_TRIDENT_ACP_ENABLED=""
_EXISTING_TRIDENT_ACP_IMAGE=""

# _PROCESSED_LABELS will contain an already indented, YAML-compliant "map" (in string form) of the given LABELS.
# Example: "    label1: value1\n    label2: value2\n    label3: value3"
_PROCESSED_LABELS=""

# ------------ CONSTANTS ------------
readonly __RELEASE_VERSION="24.02"
readonly -a __REQUIRED_TOOLS=("jq" "kubectl" "curl" "grep" "sort" "uniq" "find" "base64" "wc" "awk")

readonly __GIT_REF_CONNECTOR_OPERATOR="main" # Determines the ACOP branch from which the kustomize resources will be pulled
readonly __GIT_REF_TRIDENT="initial-manifest" # Determines the Trident branch from which the kustomize resources will be pulled

# Kustomize is 1.14+ only
readonly __KUBECTL_MIN_VERSION="1.14"

# Based on Trident requirements https://docs.netapp.com/us-en/trident/trident-get-started/requirements.html#supported-frontends-orchestrators
readonly __KUBERNETES_MIN_VERSION="1.23"
readonly __KUBERNETES_MAX_VERSION="1.29"

readonly __COMPONENTS_ALL_ASTRA_CONTROL="ALL_ASTRA_CONTROL"
readonly __COMPONENTS_TRIDENT_ONLY="TRIDENT_ONLY"
readonly __COMPONENTS_ACP_ONLY="ACP_ONLY"
readonly __COMPONENTS_VALID_VALUES=("$__COMPONENTS_ALL_ASTRA_CONTROL" "$__COMPONENTS_TRIDENT_ONLY" "$__COMPONENTS_ACP_ONLY")

readonly __DEFAULT_DOCKER_HUB_IMAGE_REGISTRY="docker.io"
readonly __DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO="netapp"
readonly __DEFAULT_ASTRA_IMAGE_REGISTRY="cr.astra.netapp.io"
readonly __DEFAULT_IMAGE_TAG="$__RELEASE_VERSION"

readonly __DEFAULT_TRIDENT_OPERATOR_IMAGE_NAME="trident-operator"
readonly __DEFAULT_TRIDENT_IMAGE_NAME="trident"
readonly __DEFAULT_CONNECTOR_OPERATOR_IMAGE_NAME="astra-connector-operator"
readonly __DEFAULT_CONNECTOR_IMAGE_NAME="astra-connector"
readonly __DEFAULT_NEPTUNE_IMAGE_NAME="controller"
readonly __DEFAULT_TRIDENT_ACP_IMAGE_NAME="trident-acp"

readonly __GENERATED_CRS_DIR="./astra-generated"
readonly __GENERATED_CRS_FILE="$__GENERATED_CRS_DIR/crs.yaml"
readonly __GENERATED_OPERATORS_DIR="$__GENERATED_CRS_DIR/operators"
readonly __GENERATED_KUSTOMIZATION_FILE="$__GENERATED_OPERATORS_DIR/kustomization.yaml"
readonly __GENERATED_PATCHES_FILE="$__GENERATED_CRS_DIR/post-deploy-patches"

readonly __DEBUG=10
readonly __INFO=20
readonly __WARN=30
readonly __ERROR=40
readonly __FATAL=50

readonly __NEWLINE=$'\n' # This is for readability

#----------------------------------------------------------------------
#-- Script config
#----------------------------------------------------------------------
get_configs() {
    # ------------ SCRIPT BEHAVIOR ------------
    CONFIG_FILE="${CONFIG_FILE:-}" # Overrides environment variables specified via command line
    DRY_RUN="${DRY_RUN:-"true"}" # Skips applying generated resources
    SKIP_IMAGE_CHECK="${SKIP_IMAGE_CHECK:-"false"}" # Skips checking whether images exist or not
    SKIP_ASTRA_CHECK="${SKIP_ASTRA_CHECK:-"false"}" # Skips AC URL, cloud ID, and cluster ID check
    # DISABLE_PROMPTS skips prompting the user when something notable is about to happen (such as a Trident Upgrade).
    # As a guardrail, setting DISABLE_PROMPTS=true will require the SKIP_TRIDENT_UPGRADE env var to also be set.
    DISABLE_PROMPTS="${DISABLE_PROMPTS:-"false"}"
    SKIP_TRIDENT_UPGRADE="${SKIP_TRIDENT_UPGRADE}" # Required if DISABLED_PROMPTS=true

    # ------------ GENERAL ------------
    KUBECONFIG="${KUBECONFIG}"
    COMPONENTS="${COMPONENTS:-$__COMPONENTS_ALL_ASTRA_CONTROL}" # Determines what we'll install/upgrade
    IMAGE_PULL_SECRET="${IMAGE_PULL_SECRET:-}" # TODO ASTRACTL-32772: skip prompt if IMAGE_REGISTRY is default
    NAMESPACE="${NAMESPACE:-}" # Overrides EVERY resource's namespace (for fresh installs only, not upgrades)
    LABELS="${LABELS:-}" # TODO ASTRACTL-32773: pass these on to all resources (see design page for env var format)

    # ------------ IMAGE REGISTRY ------------
    # The REGISTRY environment variables follow a hierarchy; each layer overwrites the previous, if specified.
    # Note: the registry should not include a repository path. For example, if an image is hosted at
    # `cr.astra.netapp.io/common/image/path/astra-connector`, then the registry should be set to
    # `cr.astra.netapp.io` and NOT `cr.astra.netapp.io/common/image/path`.
    IMAGE_REGISTRY="${IMAGE_REGISTRY}"
        DOCKER_HUB_IMAGE_REGISTRY="${DOCKER_HUB_IMAGE_REGISTRY:-${IMAGE_REGISTRY:-$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY}}"
            TRIDENT_OPERATOR_IMAGE_REGISTRY="${TRIDENT_OPERATOR_IMAGE_REGISTRY:-$DOCKER_HUB_IMAGE_REGISTRY}"
            TRIDENT_IMAGE_REGISTRY="${TRIDENT_IMAGE_REGISTRY:-$DOCKER_HUB_IMAGE_REGISTRY}"
            CONNECTOR_OPERATOR_IMAGE_REGISTRY="${CONNECTOR_OPERATOR_IMAGE_REGISTRY:-$DOCKER_HUB_IMAGE_REGISTRY}"
        ASTRA_IMAGE_REGISTRY="${ASTRA_IMAGE_REGISTRY:-${IMAGE_REGISTRY:-$__DEFAULT_ASTRA_IMAGE_REGISTRY}}"
            CONNECTOR_IMAGE_REGISTRY="${CONNECTOR_IMAGE_REGISTRY:-$ASTRA_IMAGE_REGISTRY}"
            NEPTUNE_IMAGE_REGISTRY="${NEPTUNE_IMAGE_REGISTRY:-$ASTRA_IMAGE_REGISTRY}"
            TRIDENT_ACP_IMAGE_REGISTRY="${TRIDENT_ACP_IMAGE_REGISTRY:-$ASTRA_IMAGE_REGISTRY}"

    # ------------ IMAGE REPO ------------
    # The REPO environment variables follow a hierarchy; each layer overwrites the previous, if specified.
    # Example: if all images are hosted under `cr.astra.netapp.io/common/image/repo` (with one such
    # image perhaps being `cr.astra.netapp.io/common/image/repo/astra-connector:latest`) then
    # IMAGE_BASE_REPO should be set to `common/image/repo`. To be more specific, this should be the URL
    # that can be used to access the `/v2/` endpoint. Taking the previous example, the /v2/ route would be
    # at `cr.astra.netapp.io/v2/`.
    IMAGE_BASE_REPO=${IMAGE_BASE_REPO:-""}
        DOCKER_HUB_BASE_REPO="${DOCKER_HUB_BASE_REPO:-${IMAGE_BASE_REPO:-$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO}}"
            TRIDENT_OPERATOR_IMAGE_REPO="${TRIDENT_OPERATOR_IMAGE_REPO:-"$(join_rpath "$DOCKER_HUB_BASE_REPO" "$__DEFAULT_TRIDENT_OPERATOR_IMAGE_NAME")"}"
            TRIDENT_IMAGE_REPO="${TRIDENT_IMAGE_REPO:-"$(join_rpath "$DOCKER_HUB_BASE_REPO" "$__DEFAULT_TRIDENT_IMAGE_NAME")"}"
            CONNECTOR_OPERATOR_IMAGE_REPO="${CONNECTOR_OPERATOR_IMAGE_REPO:-"$(join_rpath "$DOCKER_HUB_BASE_REPO" "$__DEFAULT_CONNECTOR_OPERATOR_IMAGE_NAME")"}"
        ASTRA_BASE_REPO="${ASTRA_BASE_REPO:-$IMAGE_BASE_REPO}"
            # As it stands, ACOP only allows modifying the connector and neptune base repo and tag, not the image name.
            [ -n "$CONNECTOR_IMAGE_REPO" ] && logwarn "CONNECTOR_IMAGE_REPO env var is set but not supported"
            [ -n "$NEPTUNE_IMAGE_REPO" ] && logwarn "NEPTUNE_IMAGE_REPO env var is set but not supported"
            CONNECTOR_IMAGE_REPO="$(join_rpath "$ASTRA_BASE_REPO" "$__DEFAULT_CONNECTOR_IMAGE_NAME")"
            NEPTUNE_IMAGE_REPO="$(join_rpath "$ASTRA_BASE_REPO" "$__DEFAULT_NEPTUNE_IMAGE_NAME")"
            TRIDENT_ACP_IMAGE_REPO="${TRIDENT_ACP_IMAGE_REPO:-"$(join_rpath "$ASTRA_BASE_REPO" "$__DEFAULT_TRIDENT_ACP_IMAGE_NAME")"}"

    # ------------ IMAGE TAG ------------
    # The TAG environment variables follow a hierarchy; each layer overwrites the previous, if specified.
    IMAGE_TAG="${IMAGE_TAG}"
        DOCKER_HUB_IMAGE_TAG="${DOCKER_HUB_IMAGE_TAG:-${IMAGE_TAG:-$__DEFAULT_IMAGE_TAG}}"
            TRIDENT_OPERATOR_IMAGE_TAG="${TRIDENT_OPERATOR_IMAGE_TAG:-$DOCKER_HUB_IMAGE_TAG}"
            TRIDENT_IMAGE_TAG="${TRIDENT_IMAGE_TAG:-$DOCKER_HUB_IMAGE_TAG}"
            CONNECTOR_OPERATOR_IMAGE_TAG="${CONNECTOR_OPERATOR_IMAGE_TAG:-$DOCKER_HUB_IMAGE_TAG}"
        ASTRA_IMAGE_TAG="${ASTRA_IMAGE_TAG:-${IMAGE_TAG:-$__DEFAULT_IMAGE_TAG}}"
            CONNECTOR_IMAGE_TAG="${CONNECTOR_IMAGE_TAG:-$ASTRA_IMAGE_TAG}"
            NEPTUNE_IMAGE_TAG="${NEPTUNE_IMAGE_TAG:-$ASTRA_IMAGE_TAG}"
            TRIDENT_ACP_IMAGE_TAG="${TRIDENT_ACP_IMAGE_TAG:-$ASTRA_IMAGE_TAG}"

    # ------------ ASTRA CONNECTOR ------------
    ASTRA_CONTROL_URL="${ASTRA_CONTROL_URL:-"astra.netapp.io"}"
    ASTRA_CONTROL_URL="${ASTRA_CONTROL_URL%/}" # Remove trailing slash
    ASTRA_API_TOKEN="${ASTRA_API_TOKEN}"
    ASTRA_ACCOUNT_ID="${ASTRA_ACCOUNT_ID}"
    ASTRA_CLOUD_ID="${ASTRA_CLOUD_ID}"
    ASTRA_CLUSTER_ID="${ASTRA_CLUSTER_ID}"
    CONNECTOR_HOST_ALIAS_IP="${CONNECTOR_HOST_ALIAS_IP:-""}"
    CONNECTOR_HOST_ALIAS_IP="${CONNECTOR_HOST_ALIAS_IP%/}" # Remove trailing slash
    CONNECTOR_SKIP_TLS_VALIDATION="${CONNECTOR_SKIP_TLS_VALIDATION:-"false"}"
}

set_log_level() {
    LOG_LEVEL="${LOG_LEVEL:-"$__INFO"}"
    [ "$LOG_LEVEL" == "debug" ] && LOG_LEVEL="$__DEBUG"
    [ "$LOG_LEVEL" == "info" ] && LOG_LEVEL="$__INFO"
    [ "$LOG_LEVEL" == "warn" ] && LOG_LEVEL="$__WARN"
    [ "$LOG_LEVEL" == "error" ] && LOG_LEVEL="$__ERROR"
    [ "$LOG_LEVEL" == "fatal" ] && LOG_LEVEL="$__FATAL"
}

load_config_from_file_if_given() {
    local config_file=$1
    if [ -z "$config_file" ]; then return 0; fi
    if [ ! -f "$config_file" ]; then
        add_problem "CONFIG_FILE '$config_file' does not exist" "Given CONFIG_FILE '$config_file' does not exist"
        return 1
    fi

    # shellcheck disable=SC1090
    source "$config_file"
    set_log_level
    logheader $__DEBUG "Loaded configuration from file: $config_file"
}

add_to_config_builder() {
    local -r var_name=$1
    if [ -z "$var_name" ]; then fatal "no var_name was given"; fi

    logdebug "$var_name='${!var_name}'"
    _CONFIG_BUILDER+=("$var_name='${!var_name}'")
}

print_built_config() {
    if [ "${#_CONFIG_BUILDER[@]}" -eq 0 ]; then return 0; fi
    echo
    echo "----------------- GENERATED CONFIG -----------------"
    printf "%s\n" "${_CONFIG_BUILDER[@]}"
    echo "----------------------------------------------------"
}

components_include_connector() {
    [ "$COMPONENTS" == "$__COMPONENTS_ALL_ASTRA_CONTROL" ] && return 0
    return 1
}

components_include_trident() {
    if [ "$COMPONENTS" == "$__COMPONENTS_ALL_ASTRA_CONTROL" ] || [ "$COMPONENTS" == "$__COMPONENTS_TRIDENT_ONLY" ]; then
        return 0
    fi
    return 1
}

components_include_acp() {
    if [ "$COMPONENTS" == "$__COMPONENTS_ALL_ASTRA_CONTROL" ] || [ "$COMPONENTS" == "$__COMPONENTS_ACP_ONLY" ]; then
        return 0
    fi
    return 1
}

trident_is_missing() {
    [ -z "$_EXISTING_TRIDENT_NAMESPACE" ] && return 0
    return 1
}

should_skip_trident_upgrade() {
    [ "$SKIP_TRIDENT_UPGRADE" == "true" ] && return 0
    return 1
}

trident_image_needs_upgraded() {
    local -r configured_image="$(as_full_image "$TRIDENT_IMAGE_REGISTRY" "$TRIDENT_IMAGE_REPO" "$TRIDENT_IMAGE_TAG")"

    logdebug "Checking if Trident image needs upgraded: $_EXISTING_TRIDENT_IMAGE vs $configured_image"
    if [ "$_EXISTING_TRIDENT_IMAGE" != "$configured_image" ]; then
        return 0
    fi

    return 1
}

trident_operator_image_needs_upgraded() {
    local -r configured_image="$(as_full_image "$TRIDENT_OPERATOR_IMAGE_REGISTRY" "$TRIDENT_OPERATOR_IMAGE_REPO" "$TRIDENT_OPERATOR_IMAGE_TAG")"

    logdebug "Checking if Trident image needs upgraded: $_EXISTING_TRIDENT_OPERATOR_IMAGE vs $configured_image"
    if [ "$_EXISTING_TRIDENT_OPERATOR_IMAGE" != "$configured_image" ]; then
        return 0
    fi

    return 1
}

acp_image_needs_upgraded() {
    local -r configured_image="$(as_full_image "$TRIDENT_ACP_IMAGE_REGISTRY" "$TRIDENT_ACP_IMAGE_REPO" "$TRIDENT_ACP_IMAGE_TAG")"

    logdebug "Checking if ACP image needs upgraded: $_EXISTING_TRIDENT_ACP_IMAGE vs $configured_image"
    if [ "$_EXISTING_TRIDENT_ACP_IMAGE" != "$configured_image" ]; then
        return 0
    fi
    return 1
}

acp_is_enabled() {
    logdebug "Checking if ACP is enabled: '$_EXISTING_TRIDENT_ACP_ENABLED'"
    [ "$_EXISTING_TRIDENT_ACP_ENABLED" == "true" ] && return 0
    return 1
}

config_image_is_custom() {
    local -r component_name="$1"
    local -r default_registry="$2"
    local -r default_base_repo="$3"

    [ -z "$component_name" ] && fatal "no component_name given"

    local -r registry_var="${component_name}_IMAGE_REGISTRY"
    local -r repo_var="${component_name}_IMAGE_REPO"
    local -r tag_var="${component_name}_IMAGE_TAG"
    local -r current_image="$(as_full_image "${!registry_var}" "${!repo_var}" "${!tag_var}")"

    local -r default_image_name_var="__DEFAULT_${component_name}_IMAGE_NAME"
    local -r default_repo="$(join_rpath "$default_base_repo" "${!default_image_name_var}")"
    local -r default_tag="$__DEFAULT_IMAGE_TAG"
    local -r default_image="$(as_full_image "$default_registry" "$default_repo" "$default_tag")"

    [ "$current_image" != "$default_image" ] && return 0
    return 1
}

config_trident_operator_image_is_custom() {
    if config_image_is_custom "TRIDENT_OPERATOR" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

config_trident_image_is_custom() {
    if config_image_is_custom "TRIDENT" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

config_connector_operator_image_is_custom() {
    if config_image_is_custom "CONNECTOR_OPERATOR" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

config_connector_image_is_custom() {
    if config_image_is_custom "CONNECTOR" "$__DEFAULT_ASTRA_IMAGE_REGISTRY" "$__DEFAULT_ASTRA_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

config_neptune_image_is_custom() {
    if config_image_is_custom "NEPTUNE" "$__DEFAULT_ASTRA_IMAGE_REGISTRY" "$__DEFAULT_ASTRA_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

config_acp_image_is_custom() {
    if config_image_is_custom "TRIDENT_ACP" "$__DEFAULT_ASTRA_IMAGE_REGISTRY" "$__DEFAULT_ASTRA_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

prompts_disabled() {
    [ "${DISABLE_PROMPTS}" == "true" ] && return 0
    return 1
}

is_dry_run() {
    [ "${DRY_RUN}" == "true" ] && return 0
    return 1
}

#----------------------------------------------------------------------
#-- Util functions
#----------------------------------------------------------------------
log_at_level() {
    if [ -z "$LOG_LEVEL" ]; then LOG_LEVEL=$__INFO; fi

    local -r given_level="${1:-$__DEBUG}"
    local msg="$2"

    if [ "$LOG_LEVEL" -eq "$__DEBUG" ]; then
        [ "$given_level" == $__DEBUG ] && msg="|DEBUG|  $msg"
        [ "$given_level" == $__INFO ] && msg="|INFO |  $msg"
        [ "$given_level" == $__WARN ] && msg="|WARN |  $msg"
        [ "$given_level" == $__ERROR ] && msg="|ERROR|  $msg"
    fi
    if [ "$given_level" -ge "$LOG_LEVEL" ]; then
        echo "$msg"
    fi
}

logln() {
    log_at_level "$1" ""
    log_at_level "$1" "$2"
}

logheader() {
    logln "$1" "--- $2"
}

logdebug() {
    log_at_level $__DEBUG "$1"
}

loginfo() {
    log_at_level $__INFO "$1"
}

logwarn() {
    log_at_level $__WARN "$1"
}

logerror() {
    log_at_level $__ERROR "$1"
}

# fatal will log the given error (followed by a stack trace) and then exit with code 1.
# Use it only for script execution errors -- for business logic errors, use `add_problem`.
fatal() {
    local -r msg="${1:-"unspecified failure"}"
    local full="FATAL: ${msg}";
    local -r previous_fn="${FUNCNAME[1]}"
    if [ -n "$previous_fn" ]; then full="FATAL: $previous_fn: ${msg}"; fi

    logln $__FATAL "$full"
    # Iterate through call stack in reverse order,
    # excluding "main" (the last entry, which has a line number of 0)
    for ((i=${#FUNCNAME[@]}-2; i>=0; i--)); do
        log_at_level $__FATAL "-> line ${BASH_LINENO[$i]}: ${FUNCNAME[$i]}"
    done

    exit 1
}

insert_into_file_after_pattern() {
    local -r filepath="$1"
    local -r pattern="$2"
    local -r insert_content="$3"
    [ ! -f "$filepath" ] && fatal "file '$filepath' does not exist"
    [ -z "$pattern" ] && fatal "no pattern given"
    [ -z "$insert_content" ] && fatal "no content to insert given"

    local -r total_lines="$(wc -l "$filepath" | awk '{print $1}')"
    local new_content=""
    local lineno=1
    local found_pattern="false"
    while IFS= read -r line
    do
        new_content+="$line"
        if echo "$line" | grep -q -- "$pattern"; then
            new_content+="${__NEWLINE}$insert_content"
            found_pattern="true"
        fi

        # Add a newline character unless it's the last line
        if [ "$lineno" -lt "$total_lines" ]; then
            new_content+=${__NEWLINE}
        fi
        lineno=$((lineno + 1))
    done < "$filepath"

    [ "$found_pattern" != "true" ] && fatal "did not find pattern '$pattern' in file"
    echo "$new_content" > "$filepath"
}

prompt_user() {
    local -r var_name="$1"
    local -r prompt_msg="$2"
    if [ -z "$var_name" ]; then fatal "no variable name was given"; fi
    if [ -z "$prompt_msg" ]; then fatal "no prompt message was given"; fi
    if [ "$DISABLE_PROMPTS" == "true" ]; then return 0; fi

    while true; do
        read -p "$prompt_msg" -r "${var_name?}"
        if [ -n "${!var_name}" ]; then
            break
        else
            logerror "Error: Input cannot be empty."
        fi
    done
}

prompt_user_yes_no() {
    local prompt=$1
    local result
    if [ -z "$prompt" ]; then fatal "no prompt message was given"; fi
    if [ "$DISABLE_PROMPTS" == "true" ]; then return 0; fi

    echo
    while true; do
        read -p "$prompt (yes/no): " result
        case $result in
            [Yy]* ) return 0;;
            [Nn]* ) return 1;;
            * ) logerror "Please answer yes or no.";;
        esac
    done
}

with_trailing_slash() {
    local str="$1"
    [ -z "$str" ] && return 0

    local str="$1"
    str="${str#/}" # Remove starting slash if present
    str="${str%/}" # Remove trailing slash if present
    str="$str/"

    echo "$str"
}

join_rpath() {
    local args=("$@")
    [ "${#args[@]}" -eq 0 ] && echo "" && return 0

    local joined=""
    for (( i = 0; i < ${#args[@]}; i+=1 )); do
        args[i]="${args[i]#/}" # Remove starting slash if present
        args[i]="${args[i]%/}" # Remove trailing slash if present
        if [ "$i" -eq 0 ]; then joined="${args[i]}"
        else joined="$joined/${args[i]}"; fi
    done

    echo "$joined"
}

get_base_repo() {
    local -r image_repo="$1"
    if [ -z "$image_repo" ]; then fatal "no image_repo given"; fi

    local -r base="$(dirname "$image_repo")"
    if [ "$base" == "." ]; then base="$image_repo"; fi

    echo "$base"
}

as_full_image() {
    local -r registry="$1"
    local -r image_repo="$2"
    local -r image_tag="$3"

    if [ -z "$image_repo" ]; then fatal "no image_repo given"; fi
    if [ -z "$image_tag" ]; then fatal "no image_tag given"; fi

    echo "$(join_rpath "$registry" "$image_repo"):$image_tag"
}

tool_is_installed() {
    local -r tool="$1"
    if [ -z "$tool" ]; then fatal "no tool name provided"; fi

    if
        command -v "$tool" &>/dev/null
        return 0
    then
        return 1
    fi
}

version_in_range() {
    local -r current=$1
    local -r min=$2
    local -r max=$3

    if [ -z "$current" ]; then fatal "no current version given"; fi
    if [ -z "$min" ]; then fatal "no min version given"; fi
    if [ -z "$max" ]; then fatal "no max version given"; fi

    local -r middle_version="$(printf "%s\n%s\n%s" "$current" "$min" "$max" | sort -V | head -n 2 | tail -n 1)"
    if [ "$middle_version" == "$current" ]; then
        return 0
    fi
    return 1
}

version_higher_or_equal() {
    local -r current=$1
    local -r min=$2

    if [ -z "$current" ]; then fatal "no current version given"; fi
    if [ -z "$min" ]; then fatal "no min version given"; fi
    [ "$current" == "$min" ] && return 0;

    local -r bottom_version=$(printf "%s\n%s" "$current" "$min" | sort -V | tail -n 1)
    if [ "$bottom_version" == "$current" ]; then
        return 0
    fi
    return 1
}

add_problem() {
    local problem_simple=$1
    local problem_long=${2:-$problem_simple}

    if [ -z "$problem_simple" ]; then fatal "no error message given"; fi

    logdebug "$problem_simple"
    _PROBLEMS+=("$problem_long")
}

make_astra_control_request() {
    local sub_path="$1"
    if [ -z "$sub_path" ]; then fatal "no sub_path given (if the base route is needed, use '/')"; fi
    if [ "$sub_path" == "/" ]; then sub_path=""; fi
    if [ -z "$ASTRA_CONTROL_URL" ]; then fatal "no ASTRA_CONTROL_URL found"; fi
    if [ -z "$ASTRA_ACCOUNT_ID" ]; then fatal "no ASTRA_ACCOUNT_ID found"; fi
    if [ -z "$ASTRA_API_TOKEN" ]; then fatal "no ASTRA_API_TOKEN found"; fi

    # TODO ASTRACTL-32772: make it so including https:// and http:// is optional in ASTRA_CONTROL_URL
    local -r url="$ASTRA_CONTROL_URL/accounts/$ASTRA_ACCOUNT_ID$sub_path"
    local -r method="GET"
    local headers=("-H" 'Content-Type: application/json')
    headers+=("-H" 'Accept: application/json')
    headers+=("-H" "Authorization: Bearer $ASTRA_API_TOKEN")

    _return_body=""
    _return_status=""
    local skip_tls_validation_opt=""
    if [ "$CONNECTOR_SKIP_TLS_VALIDATION" == "true" ]; then
        skip_tls_validation_opt="-k"
    fi

    logdebug "$method --> '$url'"
    local -r result="$(curl -X "$method" -s $skip_tls_validation_opt -w "\n%{http_code}" "$url" "${headers[@]}")"
    _return_body="$(echo "$result" | head -n 1)"
    _return_status="$(echo "$result" | tail -n 1)"
}

is_docker_hub() {
    local -r registry_url="$1"
    if [ -z "$registry_url" ]; then fatal "no registry_url given"; fi

    if echo "$registry" | grep -q "docker.io"; then
        return 0
    fi
    return 1
}

is_astra_registry() {
    local -r registry_url="$1"
    if [ -z "$registry_url" ]; then fatal "no registry_url given"; fi

    if echo "$registry" | grep -q "cr." && echo "$registry" | grep -q "astra"; then
        return 0
    fi
    return 1
}

get_registry_credentials_from_pull_secret() {
    local -r pull_secret="$1"
    local -r namespace="$2"
    if [ -z "$pull_secret" ]; then fatal "no pull_secret given"; fi
    if [ -z "$namespace" ]; then fatal "no namespace given"; fi

    # General steps for getting the username/pw from a pull secret since it involves a lot of jq operations
    # and the objects returned don't have the most obvious structure...
    #
    # 1. Get the kubectl secret as json:
    # {
    #     "apiVersion": "v1",
    #     "data": {".dockerconfigjson": "B64_ENCODED_VALUE"},
    #     ...
    # }
    #
    # 2. B64_ENCODED_VALUE, once decoded, should have the following format:
    # {
    #   "auths": {
    #     "https://my.registry.io": {
    #       "username": "B64_ENCODED_USERNAME",
    #       "password": "B64_ENCODED_PASSWORD",
    #       "auth": "B64_ENCODED_USERNAME_PASSWORD"
    #     },
    #     "https://my.other.registry.io": { ... }
    #   }
    # }
    #
    # 3. B64_ENCODED_USERNAME_PASSWORD, once decoded, is actually just the username and password together:
    # 'my_username:my_password'
    #
    # 4. List each key/value pair as "entries", using jq's `to_entries`:
    # [
    #   {
    #     "key": "https://my.registry.io",
    #     "value": {
    #       "username": "username",
    #       "password": "password",
    #       "auth": "B64_ENCODED_USERNAME_PASSWORD"
    #     }
    #   },
    #   { "key": "https://my.other.registry.io", "value": { ... } }
    # ]
    #
    # 5. Filter entries where $registry contains the `key`
    # 6. Get the first matching entry's `.value.auth`, which would be `B64_ENCODED_USERNAME_PASSWORD` here
    # 7. The `B64_ENCODED_USERNAME_PASSWORD` can then be passed to curl via
    # `-H 'Authorization: Basic B64_ENCODED_USERNAME_PASSWORD'` without having to decode it first.
    local -r contents="$(kubectl get secret "$pull_secret" -n "$namespace" -o json 2> /dev/null)"
    if [ -z "$contents" ]; then
        # TODO: differentiate between connectivity errors and actual 'not found' errors
        add_problem "pull secret '$namespace.$pull_secret': not found" "Pull secret '$pull_secret' not found in namespace '$namespace'"
        return 1
    fi

    # Note: assigning the value of `.key` to variable $k is necessary as jq doesn't allow referring to
    # paths such as `.key` in function parameters
    local -r registry_selector='(.key | contains($reg)) or (.key as $k | $reg | contains($k))'
    local -r registry_filter="map(select($registry_selector))"
    local -r encoded_creds="$( \
        echo "$contents" \
        | jq -r '.data[".dockerconfigjson"]' 2> /dev/null \
        | base64 -d 2> /dev/null \
        | jq -r '.auths' 2> /dev/null \
        | jq -r --arg reg "$registry" "to_entries | $registry_filter" 2> /dev/null \
        | jq -r '.[0].value.auth' 2> /dev/null \
    )"
    if [ -z "$encoded_creds" ] || [ "$encoded_creds" == "null" ]; then
        add_problem "pull secret '$namespace.$pull_secret': failed to extract encoded_creds" "Registry pull secret '$pull_secret' is in the wrong format."
        logdebug "result: $encoded_creds"
        return 1
    fi
    _return_value="$encoded_creds"
    return 0
}

# k8s_cluster_can_pull_from_docker_hub deploys a busybox pod and executes a simple echo command on it. This
# allows us to check whether the k8s cluster can pull from docker hub.
k8s_cluster_can_pull_from_docker_hub() {
    local namespace="${1:-"default"}"
    local -r registry="docker.io"
    local -r image_and_tag="busybox"

    if [ -z "$registry" ]; then fatal "no registry given"; fi
    if [ -z "$image_and_tag" ]; then fatal "no image given"; fi

    local -r full_image="$registry/$image_and_tag"

    logdebug "checking if '$full_image' can be pulled from the cluster (namespace: $namespace)..."

    local -r pod_name="astra-preflight-check-test"
    local pull_secret_override="[]"
    if [ -n "$pull_secret" ]; then pull_secret_override='[{"name": "'$pull_secret'"}]'; fi
    local -r spec_overrides='
    {
        "spec": {
            "imagePullSecrets": '$pull_secret_override',
            "containers": [
                {
                    "name": "test-container-override",
                    "image": "'"$full_image"'",
                    "command": ["echo", "pull test successful"]
                }
            ]
        }
    }'

    # Run actual test
    kubectl delete pod -n "$namespace" "$pod_name" &> /dev/null || true
    if kubectl run "$pod_name" --image="$full_image" \
            -n "$namespace" \
            --image-pull-policy=Always \
            --restart=Never \
            --overrides="${spec_overrides}" &> /dev/null
    then
        kubectl delete pod -n "$namespace" "$pod_name" > /dev/null || true
        return 0
    else
        local -r message="$(kubectl get pod -n "$namespace" $pod_name -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}')"
        local -r reason="$(kubectl get pod -n "$namespace" $pod_name -o jsonpath='{.status.containerStatuses[0].state.waiting.message}')"
        kubectl delete pod -n "$namespace" "$pod_name" > /dev/null || true
        add_problem "$full_image: failed (${reason:-"unknown error"})" "Failed to pull image '$full_image': ${reason:-"unknown error"}: ${message:-""}"
        return 1
    fi
}

check_if_image_can_be_pulled_via_curl() {
    local -r registry="$1"
    local -r image_repo="$2"
    local -r image_tag="$3"
    local encoded_creds="${4:-""}" # Encoded creds format: 'username:password' encoded in b64.
    _return_body=""
    _return_status=""

    if [ -z "$registry" ]; then fatal "no registry given"; fi
    if [ -z "$image_repo" ]; then fatal "no image_repo given"; fi
    if [ -z "$image_tag" ]; then fatal "no image_tag given"; fi

    local -a args=('-s' '-w' "\n%{http_code}")
    if [ -n "$encoded_creds" ]; then
        args+=("-H" "Authorization: Basic $encoded_creds")
    fi

    local -r result="$(curl -X GET "${args[@]}" "https://$registry/v2/$image_repo/tags/list")"
    local -r line_count="$(echo "$result" | wc -l)"
    _return_body="$(echo "$result" | head -n "${line_count-1}")"
    _return_status="$(echo "$result" | tail -n 1)"
    _return_error=""

    if [ "$_return_status" == 200 ]; then
        # Example response body:
        # {
        #   "name" : "my/repo/astra-connector",
        #   "tags" : [ "032172b", "074dk52", "1fbc135", "24.02" ]
        # }
        if echo "$_return_body" | grep "tags" | grep -q -- "$image_tag"; then
            return 0
        fi
        _return_error="repository found but tag '$image_tag' does not exist"
    fi
    return 1
}

process_labels_to_yaml() {
    # The labels string should have a format like this: 'label1=value1 label2=value2'
    local labels_string="$1"
    local indent="${2:-""}"
    local label_separator="${3:-' '}"
    local key_value_separator="${4:-'='}"

    [ -z "$labels_string" ] && return 0

    # Split the string on the label separator. Result:
    # ("label1=value1" "label2=value2")
    local -a pairs=()
    IFS=$label_separator read -r -a "pairs" <<EOF
$labels_string
EOF

    # Further split the labels on the key/value separator. Result:
    # ("label1" "value1" "label2" "value2")
    local -a all_keys_and_values=()
    local -a current=()
    for pair in "${pairs[@]}"; do
      IFS=$key_value_separator read -r -a "current" <<EOF
$pair
EOF
      all_keys_and_values+=("${current[@]}")
    done

    # Make sure we have an even number of values
    local -r length="${#all_keys_and_values[@]}"
    if ! [ $((length % 2)) -eq 0 ]; then echo "" && return 1; fi

    # Form the full string, including indent. Result:
    # [indent]label1: value1
    # [indent]label2: value2
    local key=""
    local value=""
    local yaml_labels=""
    for (( i = 0; i < "$length"; i+=2 )); do
        key="${all_keys_and_values[i]}"
        value="${all_keys_and_values[i+1]}"
        yaml_labels+="${indent}${key}: ${value}"

        # Add a newline character unless it's the last label
        if [ "$i" -lt "$((length - 2))" ]; then
            yaml_labels+=${__NEWLINE}
        fi
    done

    echo "$yaml_labels"
}

generate_resource_limits_patch_pre_deploy() {
    # TODO ASTRACTL-32773: resource limits
    return 0

    local -r kind="${1}"
    local -r resource_name="$2"
    local -r containers=("${@:3}")

    [ -z "$kind" ] && fatal "no kind given"
    [ -z "$resource_name" ] && fatal "no resource name given"
    [ "${#containers[@]}" -eq 0 ] && fatal "no containers given"

    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    local -r patch_file_name="resource-limits-transformer.yaml"
    local -r patch_file_path="$__GENERATED_OPERATORS_DIR/$patch_file_name"

    logheader $__DEBUG "Generating resource limits patch for '$kind/$resource_name'"

    for container in "${containers[@]}" ; do
        logdebug "container: $container"
        cat << EOF >> "$patch_file_path"
apiVersion: builtin
kind: PatchTransformer
metadata:
  name: resource-limits--${resource_name}--${container}
patch: |-
  kind: ${kind}
  metadata:
    name: "${resource_name}"
  spec:
    template:
      spec:
        containers:
        - name: ${container}
          resources:
            limits:
              cpu: "5"
              memory: "2Gi"
target:
  kind: Deployment
---
EOF
    done

    if ! grep -q -- "$patch_file_name" < "$kustomization_file"; then
        logdebug "added $patch_file_name to kustomization transformers list"
        insert_into_file_after_pattern "$kustomization_file" "transformers:" "- $patch_file_name"
    fi

    logdebug "done"
}

generate_resource_limits_patch_post_deploy() {
    # TODO ASTRACTL-32773: resource limits
    return 0

    local -r kind="${1}"
    local -r resource_name="$2"
    local -r namespace="$3"
    local -r containers=("${@:4}")

    local resources_path=""
    local patches=""
    for container in "${containers[@]}" ; do
        local resources_path="/spec/containers/$container/resources"
        if [ "$kind" != "Pod" ]; then
            resources_path="/spec/template$resources_path"
        fi
        if [ -n "$patches" ]; then
            patches+=","
        fi
         '{"op":"replace","path":"/spec/enableACP","value":true}'
    done

    _PATCHES+=("$kind $resource_name -p [$patches]")

    resource_limits_patch+=',{"op":"replace","path":"/spec/enableACP","value":true}'
}

wait_for_deployment_running() {
    local -r deployment="$1"
    local -r namespace="${2:-"default"}"
    [ -z "$deployment" ] && fatal "no deployment name given"

    local -r max_checks=12
    local counter=0
    while ((counter < max_checks)); do
        if kubectl rollout status -n "$namespace" "deploy/$deployment" &> /dev/null; then
            logdebug "deploy/$deployment is now running"
            return 0
        else
            logdebug "waiting for deploy/$deployment to be running"
            ((counter++))
            sleep 5
        fi
    done

    return 1
}

wait_for_cr_state() {
    local -r resource="$1"
    local -r path="$2"
    local -r desired_state="$3"
    local -r namespace="${4:-"default"}"
    [ -z "$resource" ] && fatal "no resource given"
    [ -z "$path" ] && fatal "no JSON path given"
    [ -z "$desired_state" ] && fatal "no desired state given"

    local -r max_checks=12
    local counter=0
    local current_state=""
    while ((counter < max_checks)); do
        current_state=$(kubectl get -n "$namespace" "$resource" -o jsonpath="{$path}" 2> /dev/null)

        if [ "$current_state" == "$desired_state" ]; then
            logdebug "resource '$resource' has reached '$desired_state'"
            return 0
        else
            logdebug "waiting for resource '$resource' (ns: $namespace) to reach '$desired_state' (currently '$current_state')"
            ((counter++))
            sleep 5
        fi
    done

    return 1
}

exit_if_problems() {
    if [ ${#_PROBLEMS[@]} -ne 0 ]; then
        [ "$LOG_LEVEL" == "$__DEBUG" ] && print_built_config
        logheader $__ERROR "Pre-flight check failed! Please resolve the following issues and try again:"
        for err in "${_PROBLEMS[@]}"; do
            logerror "* $err"
        done
        exit 1
    fi
}

#----------------------------------------------------------------------
#-- Steps
#----------------------------------------------------------------------
step_check_config() {
    # COMPONENTS and associated vars
    local trident_vars=()
    trident_vars+=("TRIDENT_OPERATOR_IMAGE_REGISTRY" "$TRIDENT_OPERATOR_IMAGE_REGISTRY")
    trident_vars+=("TRIDENT_IMAGE_REGISTRY" "$TRIDENT_IMAGE_REGISTRY")
    trident_vars+=("TRIDENT_OPERATOR_IMAGE_REPO" "$TRIDENT_OPERATOR_IMAGE_REPO")
    trident_vars+=("TRIDENT_IMAGE_REPO" "$TRIDENT_IMAGE_REPO")
    trident_vars+=("TRIDENT_OPERATOR_IMAGE_TAG" "$TRIDENT_OPERATOR_IMAGE_TAG")
    trident_vars+=("TRIDENT_IMAGE_TAG" "$TRIDENT_IMAGE_TAG")

    local connector_vars=()
    connector_vars+=("CONNECTOR_OPERATOR_IMAGE_REGISTRY" "$CONNECTOR_OPERATOR_IMAGE_REGISTRY")
    connector_vars+=("CONNECTOR_IMAGE_REGISTRY" "$CONNECTOR_IMAGE_REGISTRY")
    connector_vars+=("NEPTUNE_IMAGE_REGISTRY" "$CONNECTOR_OPERATOR_IMAGE_REGISTRY")
    connector_vars+=("CONNECTOR_OPERATOR_IMAGE_REPO" "$CONNECTOR_OPERATOR_IMAGE_REPO")
    connector_vars+=("CONNECTOR_IMAGE_REPO" "$CONNECTOR_IMAGE_REPO")
    connector_vars+=("NEPTUNE_IMAGE_REPO" "$NEPTUNE_IMAGE_REPO")
    connector_vars+=("CONNECTOR_OPERATOR_IMAGE_TAG" "$CONNECTOR_OPERATOR_IMAGE_TAG")
    connector_vars+=("CONNECTOR_IMAGE_TAG" "$CONNECTOR_IMAGE_TAG")
    connector_vars+=("NEPTUNE_IMAGE_TAG" "$NEPTUNE_IMAGE_TAG")
    connector_vars+=("ASTRA_CONTROL_URL" "$ASTRA_CONTROL_URL")
    connector_vars+=("ASTRA_API_TOKEN" "$ASTRA_API_TOKEN")
    connector_vars+=("ASTRA_ACCOUNT_ID" "$ASTRA_ACCOUNT_ID")
    connector_vars+=("ASTRA_CLOUD_ID" "$ASTRA_CLOUD_ID")
    connector_vars+=("ASTRA_CLUSTER_ID" "$ASTRA_CLUSTER_ID")

    local acp_vars=()
    acp_vars+=("TRIDENT_ACP_IMAGE_REGISTRY" "$TRIDENT_ACP_IMAGE_REGISTRY")
    acp_vars+=("TRIDENT_ACP_IMAGE_REPO" "$TRIDENT_ACP_IMAGE_REPO")
    acp_vars+=("TRIDENT_ACP_IMAGE_TAG" "$TRIDENT_ACP_IMAGE_TAG")


    # Parse COMPONENTS to determine what vars we care about
    local required_vars=("KUBECONFIG" "$KUBECONFIG")
    while true; do
        if [ "$COMPONENTS" == "$__COMPONENTS_ALL_ASTRA_CONTROL" ]; then
            required_vars+=("${trident_vars[@]}")
            required_vars+=("${acp_vars[@]}")
            required_vars+=("${connector_vars[@]}")
            break
        elif [ "$COMPONENTS" == "$__COMPONENTS_TRIDENT_ONLY" ]; then
            required_vars+=("${trident_vars[@]}")
            break
        elif [ "$COMPONENTS" == "$__COMPONENTS_ACP_ONLY" ]; then
            required_vars+=("${acp_vars[@]}")
            break
        else
            local err="COMPONENTS: invalid value '$COMPONENTS'. Pick one of (${__COMPONENTS_VALID_VALUES[*]})"
            if prompts_disabled; then
                add_problem "$err."
                break
            fi

            prompt_user COMPONENTS "$err: "
        fi
    done
    add_to_config_builder "COMPONENTS"

    # First pass to find missing values
    local key
    local value
    local missing_vars=()
    for (( i = 0; i < ${#required_vars[@]}-1; i+=2 )); do
        key="${required_vars[i]}"
        value="${required_vars[i+1]}"
        add_to_config_builder "$key"
        if [ -z "$value" ]; then missing_vars+=("$key" "$value"); fi
    done

    # Second pass to prompt for missing values or
    # (if prompts are disabled) add errors so we can exit later
    for (( i = 0; i < ${#missing_vars[@]}-1; i+=2 )); do
        key="${missing_vars[i]}"
        value="${missing_vars[i+1]}"

        if [ -z "$value" ]; then
            if prompts_disabled; then
                add_problem "$key: required"
            else
                prompt_user "$key" "$key is required. Please enter a value: "
            fi
        fi
    done

    # Env vars with special conditions
    if [ -n "$IMAGE_PULL_SECRET" ]; then
        if [ -z "$NAMESPACE" ]; then
            prompt_user "NAMESPACE" "NAMESPACE is required when specifying an IMAGE_PULL_SECRET. Please enter the namespace:"
        fi
        add_to_config_builder "IMAGE_PULL_SECRET"
    fi
    add_to_config_builder "NAMESPACE"

    if prompts_disabled; then
        if [ -z "$SKIP_TRIDENT_UPGRADE" ]; then
            local -r longer_msg="SKIP_TRIDENT_UPGRADE is required when prompts are disabled."
            add_problem "SKIP_TRIDENT_UPGRADE: required (prompts disabled)" "$longer_msg"
        fi
    fi
    add_to_config_builder "DISABLE_PROMPTS"
    add_to_config_builder "SKIP_TRIDENT_UPGRADE"

    # Fully optional env vars
    add_to_config_builder "CONNECTOR_HOST_ALIAS_IP"
    add_to_config_builder "CONNECTOR_SKIP_TLS_VALIDATION"

    if [ -n "${LABELS}" ]; then
        _PROCESSED_LABELS="$(process_labels_to_yaml "${LABELS}" "    ")"
        if [ -z "${_PROCESSED_LABELS}" ]; then
            add_problem "label processing: failed" "The given LABELS could not be parsed."
        fi
    fi
    add_to_config_builder "LABELS"
}

step_check_tools_are_installed() {
    local utils=("$@")
    for cmd in "${utils[@]}"; do
        if ! tool_is_installed "$cmd"; then
            add_problem "$cmd: missing" "$cmd is not installed."
        else
            logdebug "$cmd: OK"
        fi
    done
}

step_check_kubectl_has_kustomize_support() {
    local -r minimum="${1#v}"
    if [ -z "$minimum" ]; then fatal "no minimum version was given"; fi

    logheader $__DEBUG "Checking if kubectl version supports kustomize ($minimum+)..."

    local current; current="$(kubectl version --client -o json | jq -r '.clientVersion.gitVersion')"
    current=${current#v}
    if ! version_higher_or_equal "$current" "$minimum"; then
        add_problem "kubectl version '$current': too low" "The version of your kubectl client ($current) is too low (need $__KUBECTL_MIN_VERSION+)" \
        "(requires $minimum or up)"
    else
        logdebug "kubectl version '$current': OK"
    fi
}

step_check_k8s_version_in_range() {
    local -r minimum="${1#v}"
    local -r maximum="${2#v}"

    if [ -z "$minimum" ]; then fatal "no minimum version was given"; fi
    if [ -z "$maximum" ]; then fatal "no maximum version was given"; fi

    logheader $__DEBUG "Checking if Kubernetes version is within range ($minimum <=> $maximum)..."

    local current; current="$(kubectl version -o json | jq -r '.serverVersion.gitVersion')"
    current=${current#v}
    _KUBERNETES_VERSION="$current"
    # TODO: differentiate between a connection/timeout failure and an actual version failure
    if ! version_in_range "$current" "$minimum" "$maximum"; then
        add_problem "k8s version '$current': not within range ($minimum-$maximum)"
    else
        logdebug "k8s version '$current': OK"
    fi
}

step_check_k8s_permissions() {
    logheader $__DEBUG "Checking if the Kubernetes user has admin privileges..."

    if ! tool_is_installed "kubectl"; then
        logheader $__DEBUG "kubectl is not installed, skipping k8s permission check"
    fi

    # TODO ASTRACTL-32772: differentiate between a connection/timeout failure and an actual permission failure
    if [ "$(kubectl auth can-i '*' '*' --all-namespaces)" = "yes" ]; then
        logdebug "k8s permissions: OK"
    else
        add_problem "k8s permissions: user does not have admin privileges" "Kubernetes user does not have admin privileges"
    fi
}

step_check_namespace_exists() {
    if [ -z "$(kubectl get namespace "$NAMESPACE" -o name 2> /dev/null)" ]; then
        if [ -n "$IMAGE_PULL_SECRET" ]; then
            local err_msg="The specified namespace '$NAMESPACE' does not exist on the cluster, but IMAGE_PULL_SECRET is set."
            err_msg+=" Please create the namespace and secret, or unset IMAGE_PULL_SECRET."
            add_problem "namespace '$NAMESPACE': not found but IMAGE_PULL_SECRET is set" "$err_msg"
            exit_if_problems
        fi
        if prompt_user_yes_no "The namespace $NAMESPACE doesn't exist. Create it now? Choosing 'no' will exit"; then
            kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
            logdebug "namespace '$NAMESPACE': OK"
        else
            local -r err_msg="User chose not to create the namespace. Please create the namespace and/or run the script again."
            add_problem "User chose not to create the namespace. Exiting." "$err_msg"
            exit_if_problems
        fi
    fi
}

step_check_all_images_can_be_pulled() {
    # This step will run one of two different tests based on whether the nature of the images:
    # - If the given image is the default (i.e. the user didn't change it), we skip any checks
    # - If the given image is custom, we do a simple cURL "does the image exist" check
    # - If any of the images are from docker hub (default or custom), we run the k8s busybox check once
    #   (no need to run it more than that)
    #
    # Ideally, we would run the 'busybox' test (or something similar) for every different registry,
    # but this is not possible for unknown registries since we can't guarantee that busybox (or an equivalent image)
    # exists, and almost all of the images we do know about (trident, astra-connector, etc.) all use distroless
    # images that have absolutely no tooling/utilities with which we can run this test. This means we'd have to run
    # the original entrypoint (i.e. run the built binary), which is risky since there may be side effects.
    #
    # TODO: It might be possible to run the test and let the pod crash, after which we examine the pod's events or
    # status to determine if the pull itself was successful. Worth investigating at some point.
    local images_to_check=() # Format: ("REGISTRY" "REPO" "TAG" "default|custom")
    local -r custom="custom"
    local -r default="default"
    if components_include_trident; then
        images_to_check+=("$TRIDENT_OPERATOR_IMAGE_REGISTRY" "$TRIDENT_OPERATOR_IMAGE_REPO" "$TRIDENT_OPERATOR_IMAGE_TAG")
        if config_trident_operator_image_is_custom; then images_to_check+=("$custom")
        else images_to_check+=("$default"); fi

        images_to_check+=("$TRIDENT_IMAGE_REGISTRY" "$TRIDENT_IMAGE_REPO" "$TRIDENT_IMAGE_TAG")
        if config_trident_image_is_custom; then images_to_check+=("$custom")
        else images_to_check+=("$default"); fi
    fi
    if components_include_connector; then
        images_to_check+=("$CONNECTOR_OPERATOR_IMAGE_REGISTRY" "$CONNECTOR_OPERATOR_IMAGE_REPO" "$CONNECTOR_OPERATOR_IMAGE_TAG")
        if config_connector_operator_image_is_custom; then images_to_check+=("$custom")
        else images_to_check+=("$default"); fi

        images_to_check+=("$CONNECTOR_IMAGE_REGISTRY" "$CONNECTOR_IMAGE_REPO" "$CONNECTOR_IMAGE_TAG")
        if config_connector_image_is_custom; then images_to_check+=("$custom")
        else images_to_check+=("$default"); fi

        images_to_check+=("$NEPTUNE_IMAGE_REGISTRY" "$NEPTUNE_IMAGE_REPO" "$NEPTUNE_IMAGE_TAG")
        if config_neptune_image_is_custom; then images_to_check+=("$custom")
        else images_to_check+=("$default"); fi
    fi
    if components_include_acp; then
      images_to_check+=("$TRIDENT_ACP_IMAGE_REGISTRY" "$TRIDENT_ACP_IMAGE_REPO" "$TRIDENT_ACP_IMAGE_TAG")
        if config_acp_image_is_custom; then images_to_check+=("$custom")
        else images_to_check+=("$default"); fi
    fi

    logheader $__INFO "Astra images:"
    for (( i = 0; i < ${#images_to_check[@]}-1; i+=4 )); do
        loginfo "* $(as_full_image "${images_to_check[i]}" "${images_to_check[i+1]}" "${images_to_check[i+2]}")"
    done

    logheader $__INFO "Verifying registry and image access (this may take a minute)..."
    # Get credentials from the specified pull secret (if specified)
    local pull_secret_credentials=""
    if [ -n "$IMAGE_PULL_SECRET" ]; then
        logdebug "extracting credentials from pull secret: $IMAGE_PULL_SECRET"
        get_registry_credentials_from_pull_secret "$IMAGE_PULL_SECRET" "$NAMESPACE"
        pull_secret_credentials="$_return_value"
        exit_if_problems
    fi

    # Run the image check for custom images; skip the default ones
    local need_to_run_docker_hub_check="false"
    for (( i = 0; i < ${#images_to_check[@]}; i+=4 )); do
        local registry="${images_to_check[i]}"
        local image_repo="${images_to_check[i+1]}"
        local tag="${images_to_check[i+2]}"
        local default_or_custom="${images_to_check[i+3]}"
        local credentials=""
        local full_image="$(as_full_image "$registry" "$image_repo" "$tag")"
        if is_docker_hub "$registry"; then need_to_run_docker_hub_check="true"; fi

        # Skip check if it's a default image since the requests do take a while sometimes
        logheader $__DEBUG "$full_image"
        if [ "$default_or_custom" == "$default" ]; then
            logdebug "skipping default image"
            continue
        fi

        # Get credentials based on the registry and whether we have a pull secret or not
        if is_astra_registry "$registry"; then
            logdebug "astra registry detected, generating credentials from ASTRA_ACCOUNT_ID and ASTRA_API_TOKEN"
            credentials="$(echo "$ASTRA_ACCOUNT_ID:$ASTRA_API_TOKEN" | base64)"
        else
            logdebug "generic registry fallback, using credentials from pull secret '$IMAGE_PULL_SECRET'"
            credentials="$pull_secret_credentials"
        fi

        # Check that the images actually exist
        if ! check_if_image_can_be_pulled_via_curl "$registry" "$image_repo" "$tag" "$credentials"; then
            local status="$_return_status"
            local body="$_return_body"
            local error="$_return_error"

            if [ -n "$error" ]; then
                add_problem "'$full_image': $error"
            elif is_docker_hub "$registry" && [ "$status" != 200 ] && [ "$status" != 404 ]; then
                # Note on docker hub: this check is purely "best effort" and we don't fail the script even
                # if the check itself failed, unless we specifically get a 200 (success) or a 404 (not found).
                #
                # This is because of two reasons:
                #   1. Images from docker hub are generally public, and so we can't really assume that the credentials
                #   provided via the pull secret were meant for the docker hub images. We could do more advanced logic
                #   in the future (or allow more customizable pull secret vars), but this is what we have to work
                #   with for now.
                #
                #   2. The first reason wouldn't matter if it wasn't for the fact that docker hub DOES require
                #   authentication to GET/LIST tags for its images, even if pulling that image does not.
                #
                # And so, we'll try to check the image tag using the pull secret credentials (if provided),
                # and it's great if it succeeds, but it's generally expected to fail.
                logwarn "* Cannot guarantee the existence of custom Docker Hub image '$full_image', moving on."
            elif [ "$status" == 401 ]; then
                add_problem "$full_image: unauthorized ($status)"
            elif [ "$status" == 404 ]; then
                add_problem "$full_image: not found ($status)"
            else
                add_problem "$full_image: unknown error ($status)"
            fi
            logdebug "body: '$body'"
        else
            logdebug "success"
        fi
    done

    if [ "$need_to_run_docker_hub_check" == "true" ]; then
        logheader $__DEBUG "Checking if cluster can pull from docker hub"
        k8s_cluster_can_pull_from_docker_hub "$NAMESPACE"
    fi
}

step_check_astra_control_reachable() {
    logheader $__INFO "Checking if Astra Control is reachable at '$ASTRA_CONTROL_URL'..."
    make_astra_control_request "/"
    if [ "$_return_status" == 200 ]; then
        logdebug "astra control: OK"
    else
        add_problem "astra control: failed ($_return_status)" "Failed to contact Astra Control (status code: $_return_status)"
        return 1
    fi
}

step_check_astra_cloud_and_cluster_id() {
    make_astra_control_request "/topology/v1/clouds/$ASTRA_CLOUD_ID"
    if [ "$_return_status" == 200 ]; then
        logdebug "astra control cloud_id: OK"
    else
        add_problem "astra control cloud_id: failed ($_return_status)" "Given ASTRA_CLOUD_ID did not pass validation (status code: $_return_status)"
        return 1
    fi

    make_astra_control_request "/topology/v1/clouds/$ASTRA_CLOUD_ID/clusters/$ASTRA_CLUSTER_ID"
    if [ "$_return_status" == 200 ]; then
        logdebug "astra control cluster_id: OK"
    else
        add_problem "astra control: failed ($_return_status)" "Given ASTRA_CLUSTER_ID did not pass validation (status code: $_return_status)"
        return 1
    fi
}

step_check_kubeconfig_choice() {
    if ! prompt_user_yes_no "Astra Components will be installed using kubeconfig '$KUBECONFIG'. Proceed?"; then
        echo "* User chose not to proceed. To change the kubeconfig, set the KUBECONFIG environment variable" \
        "and run the script again."
        exit 0
    fi
}

step_init_generated_dirs_and_files() {
    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    local -r kustomization_dir="$(dirname "$kustomization_file")"
    local -r crs_file="$__GENERATED_CRS_FILE"
    local -r crs_dir="$(dirname "$crs_file")"

    logheader $__DEBUG "Initializing generated directories and files"
    logdebug "$kustomization_file"
    logdebug "$crs_file"

    rm -rf "$kustomization_dir" "$crs_dir"
    mkdir -p "$crs_dir"
    mkdir -p "$kustomization_dir"

    if [ ! -f "$kustomization_file" ]; then
        cat <<EOF > "$kustomization_file"
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
patches:
transformers:
EOF
        logdebug "$kustomization_file: OK"
    fi
}

step_kustomize_global_namespace_if_needed() {
    local -r global_namespace="${1:-""}"
    local -r kustomization_file="${2}"
    local -r kustomization_dir="$(dirname "$kustomization_file")"
    [ -z "$global_namespace" ] && return 0

    [ -z "$kustomization_file" ] && fatal "no kustomization file given"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"

    # Add kustomization to set metadata.namespace where it's not already set
    echo "namespace: $global_namespace" >> "$kustomization_file"
    logdebug "$kustomization_file: added namespace field ($global_namespace)"

    # This transformer is more forceful than the `namespace` field above but is necessary to set
    # the namespace on a few resources that aren't caught by the former (the subjects in rolebindings for example).
    local -r transformer_file_name="namespace-transformer.yaml"
    cat <<EOF > "$kustomization_dir/$transformer_file_name"
apiVersion: builtin
kind: NamespaceTransformer
metadata:
  name: thisFieldDoesNotActuallyMatterForTransformers
  namespace: "${global_namespace}"
fieldSpecs:
- path: metadata/namespace
  create: true
EOF
    insert_into_file_after_pattern "$kustomization_file" "transformers:" "- $transformer_file_name"
    logdebug "$kustomization_file: added namespace transformer ($transformer_file_name)"
}

step_generate_astra_connector_yaml() {
    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    local -r kustomization_dir="$(dirname "$kustomization_file")"
    local -r crs_file="$__GENERATED_CRS_FILE"

    [ ! -d "$kustomization_dir" ] && fatal "kustomization directory '$kustomization_dir' does not exist"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"
    [ ! -f "$crs_file" ] && touch "$crs_file"

    logheader $__DEBUG "Generating Astra Connector YAML files..."

    local -r global_namespace="$NAMESPACE"
    local -r connector_namespace="${NAMESPACE:-"astra-connector"}"
    local -r connector_regcred_name="${IMAGE_PULL_SECRET:-"astra-connector-regcred"}"
    local -r connector_registry="$(join_rpath "$CONNECTOR_IMAGE_REGISTRY" "$(get_base_repo "$CONNECTOR_IMAGE_REPO")")"
    local -r connector_tag="$CONNECTOR_IMAGE_TAG"
    local -r neptune_tag="$NEPTUNE_IMAGE_TAG"
    local -r host_alias_ip="$CONNECTOR_HOST_ALIAS_IP"
    local -r skip_tls_validation="$CONNECTOR_SKIP_TLS_VALIDATION"
    local -r account_id="$ASTRA_ACCOUNT_ID"
    local -r cloud_id="$ASTRA_CLOUD_ID"
    local -r cluster_id="$ASTRA_CLUSTER_ID"
    local -r astra_url="$ASTRA_CONTROL_URL"
    local -r api_token="$ASTRA_API_TOKEN"
    local -r username="$account_id"
    local -r password="$api_token"
    local -r encoded_creds=$(echo -n "$username:$password" | base64)

    insert_into_file_after_pattern "$kustomization_file" "resources:" \
        "- https://github.com/NetApp/astra-connector-operator/cluster-install/?ref=$__GIT_REF_CONNECTOR_OPERATOR"
    logdebug "$kustomization_file: added resources entry for connector kustomization"

    # NATLESS feature flag patch
    local -r natless_patch="natless_patch.yaml"
    # We need to specify the original namespace (as it is set in the Astra Connector Operator repo)
    # because namespace overrides are applied AFTER patches (such as this one), so the namespace won't be
    # modified yet.
    local -r original_acop_namespace="system"
    cat <<EOF > "$kustomization_dir/$natless_patch"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: operator-controller-manager
  namespace: $original_acop_namespace
spec:
  template:
    spec:
      containers:
        - name: manager
          env:
            - name: ACOP_FEATUREFLAGS_NATLESS
              value: "true"
EOF
    insert_into_file_after_pattern "$kustomization_file" "patches:" "- path: ./$natless_patch"

    # SECRET GENERATOR
    cat <<EOF >> "$kustomization_file"
generatorOptions:
  disableNameSuffixHash: true
secretGenerator:
- name: astra-api-token
  namespace: "${connector_namespace}"
  literals:
  - apiToken="${api_token}"
EOF
    if [ -z "$IMAGE_PULL_SECRET" ]; then
        cat <<EOF >> "$kustomization_file"
- name: "${connector_regcred_name}"
  namespace: "${connector_namespace}"
  type: kubernetes.io/dockerconfigjson
  literals:
  - |
    .dockerconfigjson={
      "auths": {
        "${connector_registry}": {
          "username": "${username}",
          "password": "${password}",
          "auth": "${encoded_creds}"
        }
      }
    }
EOF
    fi
    logdebug "$kustomization_file: added secrets"

    # ASTRA CONNECTOR CR
    local labels_field_and_content=""
    if [ -n "$_PROCESSED_LABELS" ]; then
        labels_field_and_content="${__NEWLINE}  labels:${__NEWLINE}${_PROCESSED_LABELS}"
    fi
    cat <<EOF > "$crs_file"
apiVersion: astra.netapp.io/v1
kind: AstraConnector
metadata:
  name: astra-connector
  namespace: "${connector_namespace}"${labels_field_and_content}
spec:
  astra:
    accountId: ${account_id}
    tokenRef: astra-api-token
    cloudId: ${cloud_id}
    clusterId: ${cluster_id}
    skipTLSValidation: ${skip_tls_validation}  # Should be set to false in production environments
  imageRegistry:
    name: "${connector_registry}"
    secret: "${connector_regcred_name}"
  astraConnect:
    image: "${connector_tag}" # This field sets the tag, not the image
  neptune:
    image: "${neptune_tag}" # This field sets the tag, not the image
  autoSupport:
    enrolled: true
  natsSyncClient:
    cloudBridgeURL: ${astra_url}
EOF
    if [ -n "$host_alias_ip" ]; then
        echo "    hostAliasIP: $host_alias_ip" >> "$crs_file"
    fi
    echo "---" >> "$crs_file"

    logdebug "$crs_file: OK"
    logdebug "$crs_file: added AstraConnector CR"

    generate_resource_limits_patch_pre_deploy "Deployment" "operator-controller-manager" "manager" "kube-rbac-proxy"
}

step_collect_existing_trident_info() {
    logheader $__INFO "Checking if Trident is installed..."
    local -r torc_crd="$(kubectl get crd tridentorchestrators.trident.netapp.io -o name 2> /dev/null)"
    if [ -z "$torc_crd" ]; then
        logdebug "tridentorchestrator crd: not found"
        loginfo "* Trident installation not found."
        return 0
    else
        logdebug "tridentorchestrator crd: OK"
    fi

    local -r torc_json="$(kubectl get tridentorchestrator -A -o jsonpath="{.items[0]}" 2> /dev/null)"
    if [ -n "$torc_json" ]; then
        local -r trident_ns="$(echo "$torc_json" | jq -r '.spec.namespace')"
        _EXISTING_TORC_NAME="$(echo "$torc_json" | jq -r '.metadata.name')"
        _EXISTING_TRIDENT_NAMESPACE="$trident_ns"
        logdebug "trident install: OK"
        logdebug "trident namespace: $trident_ns"

        local -r trident_image="$(echo "$torc_json" | jq -r ".spec.tridentImage" 2> /dev/null)"
        if [ -n "$trident_image" ]; then
            _EXISTING_TRIDENT_IMAGE="$trident_image"
            logdebug "trident image: $trident_image"
        else
            logdebug "trident image: not found"
        fi

        local -r acp_enabled="$(echo "$torc_json" | jq -r '.spec.enableACP' 2> /dev/null)"
        if [ "$acp_enabled" == "true" ]; then
            logdebug "trident ACP enabled: yes"
            _EXISTING_TRIDENT_ACP_ENABLED="true"
        else
            _EXISTING_TRIDENT_ACP_ENABLED="false"
            logdebug "trident ACP enabled: no"
        fi

        local -r acp_image="$(echo "$torc_json" | jq -r '.spec.acpImage' 2> /dev/null)"
        if [ -n "$acp_image" ]; then
            logdebug "trident ACP image: $acp_image"
            _EXISTING_TRIDENT_ACP_IMAGE="$acp_image"
        else
            logdebug "trident ACP image: not found"
        fi
    else
        logdebug "trident install: not found"
    fi
}

step_generate_trident_yaml() {
    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    local -r crs_file="$__GENERATED_CRS_FILE"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"
    [ ! -f "$crs_file" ] && touch "$crs_file"

    logheader $__DEBUG "Generating Trident YAML files..."

    # TODO ASTRACTL-32183: use _KUBERNETES_VERSION to choose the right bundle yaml (pre 1.25 or post 1.25)
    insert_into_file_after_pattern "$kustomization_file" "resources:" \
        "- https://github.com/NetApp-Polaris/astra-deploy/trident-installer/deploy?ref=$__GIT_REF_TRIDENT"
    logdebug "$kustomization_file: added resources entry for trident operator"

    local -r trident_image="$TRIDENT_IMAGE_REGISTRY/$TRIDENT_IMAGE_REPO:$TRIDENT_IMAGE_TAG"
    local -r acp_image="$TRIDENT_ACP_IMAGE_REGISTRY/$TRIDENT_ACP_IMAGE_REPO:$TRIDENT_ACP_IMAGE_TAG"
    local pull_secret="[]"
    local namespace="trident"
    local enable_acp="true"
    local labels_field_and_content=""
    if [ -n "$IMAGE_PULL_SECRET" ]; then pull_secret='["'$IMAGE_PULL_SECRET'"]'; fi
    if [ -n "$NAMESPACE" ]; then namespace=$NAMESPACE; fi
    if [ -n "$PROCESSED_LABELS" ]; then
        labels_field_and_content="${__NEWLINE}  labels:${__NEWLINE}${_PROCESSED_LABELS}"
    fi

    cat <<EOF >> "$crs_file"
apiVersion: trident.netapp.io/v1
kind: TridentOrchestrator
metadata:
  name: "trident"
  namespace: "${namespace}"${labels_field_and_content}
spec:
  namespace: "${namespace}"
  tridentImage: "${trident_image}"
  imagePullSecrets: ${pull_secret}
  enableACP: ${enable_acp}
  acpImage: "${acp_image}"
---
EOF
    logdebug "$crs_file: added TridentOrchestrator CR"

    generate_resource_limits_patch_pre_deploy "Deployment" "trident-operator" "trident-operator"
}

step_generate_acp_patch() {
    local -r torc_name="$1"
    local -r enable_acp="${2:-"false"}"
    if [ -z "$torc_name" ]; then fatal "no trident orchestrator name was given"; fi

    logheader $__DEBUG "Generating ACP patch"

    local -r acp_image="$(as_full_image "$TRIDENT_ACP_IMAGE_REGISTRY" "$TRIDENT_ACP_IMAGE_REPO" "$TRIDENT_ACP_IMAGE_TAG")"
    local -r set_acp_image_patch='{"op":"replace","path":"/spec/acpImage","value":"'"$acp_image"'"}'
    local acp_patches="$set_acp_image_patch"
    if [ "$enable_acp" == "true" ]; then
        acp_patches+=',{"op":"replace","path":"/spec/enableACP","value":true}'
    fi

    _PATCHES+=("tridentorchestrator $torc_name -p [$acp_patches]")
}

step_add_labels_to_kustomization() {
    local processed_labels="${1:-""}"
    local -r kustomization_file="${2}"
    local -r crs_file="${3}"

    [ -z "${processed_labels}" ] && return 0
    [ -z "${kustomization_file}" ] && fatal "no kustomization file given"
    [ -z "${crs_file}" ] && fatal "no kustomization file given"
    [ ! -f "${kustomization_file}" ] && fatal "kustomization file '${kustomization_file}' does not exist"
    [ ! -f "${crs_file}" ] && fatal "crs file '${crs_file}' does not exist"

    logheader $__DEBUG "Adding labels to kustomization and crs file"

    local -r content="labels:${__NEWLINE}- pairs:${__NEWLINE}${processed_labels}"
    insert_into_file_after_pattern "${kustomization_file}" "kind:" "${content}"

    logdebug "kustomization labels: OK"
}

step_add_image_remaps_to_kustomization() {
    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"
    logheader $__DEBUG "Adding image remaps to kustomization"

    echo "images:" >> "$kustomization_file"
    if components_include_trident || trident_operator_image_needs_upgraded; then
        # Trident operator
        echo "- name: netapp/$__DEFAULT_TRIDENT_OPERATOR_IMAGE_NAME" >> "$kustomization_file"
        echo "  newName: $(join_rpath "$TRIDENT_OPERATOR_IMAGE_REGISTRY" "$TRIDENT_OPERATOR_IMAGE_REPO")" >> "$kustomization_file"
        echo "  newTag: \"$TRIDENT_OPERATOR_IMAGE_TAG\"" >> "$kustomization_file"
        logdebug "$kustomization_file: added Trident Operator image remap"
    fi
    if components_include_connector; then
        # Connector operator
        echo "- name: netapp/$__DEFAULT_CONNECTOR_OPERATOR_IMAGE_NAME" >> "$kustomization_file"
        echo "  newName: $(join_rpath "$CONNECTOR_OPERATOR_IMAGE_REGISTRY" "$CONNECTOR_OPERATOR_IMAGE_REPO")" >> "$kustomization_file"
        echo "  newTag: \"$CONNECTOR_OPERATOR_IMAGE_TAG\"" >> "$kustomization_file"
        logdebug "$kustomization_file: added Astra Connector Operator image remap"
        logdebug "$kustomization_file: added Astra Neptune image remap"
    fi
}

step_apply_resources() {
    if [ ! -f "$__GENERATED_KUSTOMIZATION_FILE" ]; then fatal "file $__GENERATED_KUSTOMIZATION_FILE not found"; fi
    local -r crs_dir="$__GENERATED_CRS_DIR"
    local -r crs_file_path="$__GENERATED_CRS_FILE"
    local -r patches_file_path="$__GENERATED_PATCHES_FILE"
    local -r operators_dir="$__GENERATED_OPERATORS_DIR"

    for p in "${_PATCHES[@]}" ; do
        echo "kubectl apply $p --type=json" >> "$patches_file_path"
    done

    logheader "$__INFO" "Applying resources..."
    logln "$__DEBUG" "Operator resources"
    _output="$(kubectl apply -k "$operators_dir")"
    logdebug "$_output"
    loginfo "* Astra operators have been applied to the cluster."

    if [ -f "$crs_file_path" ]; then
        logln "$__DEBUG" "CRs"
        if grep -q "AstraConnector" $crs_file_path; then
            logdebug "delete previous astraconnect if it exists"
            # Operator doesn't change the astraconnect spec right now so we need to delete it first if it exists
            kubectl delete -n "${NAMESPACE:-"astra-connector"}" deploy/astraconnect &> /dev/null
        fi
        _output="$(kubectl apply -f "$crs_file_path")"
        logdebug "$_output"
    else
        logdebug "No CRs file to apply"
    fi

    loginfo "* Astra CRs have been applied to the cluster."
    logheader "$__DEBUG" "Patches"
    for p in "${_PATCHES[@]}" ; do
        kubectl patch $p --type=json
    done
}

step_monitor_deployment_progress() {
    local -r global_ns="$NAMESPACE"
    local -r connector_operator_ns="${global_ns:-"astra-connector-operator"}"
    local -r connector_ns="${global_ns:-"astra-connector"}"
    local -r trident_ns="${_EXISTING_TRIDENT_NAMESPACE:-${global_ns:-"trident"}}"

    logheader "$__INFO" "Monitoring deployment progress..."
    if components_include_connector; then
        if ! kubectl rollout status deployment/operator-controller-manager -n "$connector_operator_ns" 2> /dev/null; then
            add_problem "connector operator deploy: failed" "The Astra Connector Operator failed to deploy"
        elif ! kubectl rollout status deployment/neptune-controller-manager -n "$connector_ns" 2> /dev/null; then
            add_problem "neptune deploy: failed" "Neptune failed to deploy"
        elif ! wait_for_deployment_running "astraconnect" "$connector_ns"; then
            add_problem "astraconnect deploy: failed" "The Astra Connector failed to deploy"
        elif ! wait_for_cr_state "astraconnectors/astra-connector" ".status.natsSyncClient.status" "Registered with Astra" "$connector_ns"; then
            add_problem "cluster registration: failed" "Cluster registration failed"
        fi
    fi

    if components_include_trident || components_include_acp; then
        if ! kubectl rollout status deployment/trident-operator -n "$trident_ns"; then
            add_problem "trident operator: failed" "The Trident Operator failed to deploy"
        elif ! kubectl rollout status deployment/trident-controller -n "$trident_ns"; then
            add_problem "trident controller: failed" "Trident failed to deploy"
        fi
    fi

    loginfo "(TODO)"
}

#======================================================================
#== Main
#======================================================================
set_log_level
logln $__INFO "====== Astra Cluster Installer v0.0.1 ======"
load_config_from_file_if_given "$CONFIG_FILE"
exit_if_problems

# ------------ PREFLIGHT CHECKS ------------
# CONFIG validation
get_configs
step_check_config
exit_if_problems

# TOOLS validation
logheader $__INFO "Checking if all required tools are installed..."
step_check_tools_are_installed "${__REQUIRED_TOOLS[@]}"
exit_if_problems

# K8S / KUBECTL validation
logheader $__INFO "Checking your Kubernetes version..."
step_check_kubectl_has_kustomize_support "$__KUBECTL_MIN_VERSION"
step_check_k8s_version_in_range "$__KUBERNETES_MIN_VERSION" "$__KUBERNETES_MAX_VERSION"
step_check_k8s_permissions
exit_if_problems

# REGISTRY access
if [ -n "$NAMESPACE" ]; then step_check_namespace_exists; fi
if [ "$SKIP_IMAGE_CHECK" != "true" ]; then
    step_check_all_images_can_be_pulled
else
    logdebug "skipping registry and image check (SKIP_IMAGE_CHECK=true)"
fi

# ASTRA CONTROL access
if [ "$SKIP_ASTRA_CHECK" != "true" ]; then
    step_check_astra_control_reachable
    step_check_astra_cloud_and_cluster_id
else
    logdebug "skipping all Astra checks (SKIP_ASTRA_CHECK=true)"
fi
exit_if_problems

# ------------ YAML GENERATION ------------
step_check_kubeconfig_choice
step_init_generated_dirs_and_files
step_kustomize_global_namespace_if_needed "$NAMESPACE" "$__GENERATED_KUSTOMIZATION_FILE"

# CONNECTOR yaml
if components_include_connector; then
    step_generate_astra_connector_yaml "$__GENERATED_CRS_DIR" "$__GENERATED_OPERATORS_DIR"
fi

# TRIDENT / ACP yaml
step_collect_existing_trident_info
if trident_is_missing; then
    step_generate_trident_yaml
elif components_include_trident && trident_image_needs_upgraded; then
    if should_skip_trident_upgrade; then
        loginfo "Skipping Trident upgrade (SKIP_TRIDENT_UPGRADE=true)."
    elif config_trident_image_is_custom || prompt_user_yes_no "Would you like to upgrade Trident?"; then
        step_generate_trident_yaml
    else
        loginfo "Trident will not be upgraded."
    fi
elif ! acp_is_enabled; then
    if config_acp_image_is_custom || prompt_user_yes_no "Would you like to enable ACP?"; then
        step_generate_acp_patch "$_EXISTING_TORC_NAME" "true"
    else
        loginfo "ACP will not be enabled."
    fi
elif config_acp_image_is_custom && acp_image_needs_upgraded; then
    step_generate_acp_patch "$_EXISTING_TORC_NAME" "false"
fi

# IMAGE REMAPS, LABELS, RESOURCE LIMITS yaml
step_add_labels_to_kustomization "${_PROCESSED_LABELS}" "${__GENERATED_KUSTOMIZATION_FILE}" "${__GENERATED_CRS_FILE}"
step_add_image_remaps_to_kustomization
exit_if_problems

# ------------ DEPLOYMENT ------------
if ! is_dry_run; then
    step_apply_resources
    step_monitor_deployment_progress
    exit_if_problems
    logheader $__INFO "Cluster management complete!"
else
    logheader $__INFO "DRY_RUN is ON: See generated files"
    loginfo "$(find "$__GENERATED_CRS_DIR" -type f)"
    _msg="You can run 'kustomize build $__GENERATED_OPERATORS_DIR > $__GENERATED_OPERATORS_DIR/resources.yaml'"
    _msg+=" to view the CRDs and operator resources that will be applied."
    logln $__INFO "$_msg"
    exit 0
fi

