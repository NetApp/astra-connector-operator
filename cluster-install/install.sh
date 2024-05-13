#!/bin/bash

# -- Resources
# Design Page (detailed): https://confluence.ngage.netapp.com/pages/viewpage.action?pageId=805543984
# Deploy Trident Operator: https://docs.netapp.com/us-en/trident/trident-get-started/kubernetes-deploy-operator.html
# Upgrade Trident: https://docs.netapp.com/us-en/trident/trident-managing-k8s/upgrade-trident.html

# TODO ASTRACTL-32773: do more testing with NAMESPACE="" once we a more stable kustomize structure in ACOP/Trident repos
# TODO ASTRACTL-32772: more in-depth CLOUD_ID and CLUSTER_ID check (duplicate cluster and such, see registration.go)
# TODO ASTRACTL-32138 (dependency): use _KUBERNETES_VERSION to choose the right bundle yaml (pre 1.25 or post 1.25)

#----------------------------------------------------------------------
#-- Private vars/constants
#----------------------------------------------------------------------

# ------------ STATEFUL VARIABLES ------------
_PROBLEMS=() # Any failed checks will be added to this array and the program will exit at specific checkpoints if not empty
_CONFIG_BUILDER=() # Contains the fully resolved config to be output at the end during a dry run
_KUBERNETES_VERSION=""

_EXISTING_TORC_NAME="" # TORC is short for TridentOrchestrator (works with kubectl too)
_EXISTING_TRIDENT_NAMESPACE=""
_EXISTING_TRIDENT_IMAGE=""
_EXISTING_TRIDENT_ACP_ENABLED=""
_EXISTING_TRIDENT_ACP_IMAGE=""
_EXISTING_TRIDENT_OPERATOR_IMAGE=""

# _PATCHES_ variables contain the k8s patches that will be applied after we've applied all CRs and kustomize resources.
# Entries should omit the 'kubectl patch' from the command, e.g. `deploy/astraconnect -n astra --type=json -p '[...]'`
_PATCHES_TORC=() # Patches for the TridentOrchestrator
_PATCHES_TRIDENT_OPERATOR=() # Patches for the Trident Operator

# _PROCESSED_LABELS will contain an already indented, YAML-compliant "map" (in string form) of the given LABELS.
# Example: "    label1: value1\n    label2: value2\n    label3: value3"
_PROCESSED_LABELS=""

# _PROCESSED_RESOURCE_LIMITS will contain the JSON form of the resource limits, e.g. `{"cpu": "3", "memory": "6Gi"}`
_PROCESSED_RESOURCE_LIMITS=""

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
readonly __COMPONENTS_TRIDENT_AND_ACP="TRIDENT_AND_ACP"
readonly __COMPONENTS_TRIDENT_ONLY="TRIDENT_ONLY"
readonly __COMPONENTS_ACP_ONLY="ACP_ONLY"
readonly __COMPONENTS_VALID_VALUES=("$__COMPONENTS_ALL_ASTRA_CONTROL" "$__COMPONENTS_TRIDENT_AND_ACP" \
    "$__COMPONENTS_TRIDENT_ONLY" "$__COMPONENTS_ACP_ONLY")

readonly __DEFAULT_DOCKER_HUB_IMAGE_REGISTRY="docker.io"
readonly __DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO="netapp"
readonly __DEFAULT_ASTRA_IMAGE_REGISTRY="cr.astra.netapp.io"
readonly __DEFAULT_IMAGE_TAG="$__RELEASE_VERSION"

readonly __DEFAULT_TRIDENT_OPERATOR_IMAGE_NAME="trident-operator"
readonly __DEFAULT_TRIDENT_AUTOSUPPORT_IMAGE_NAME="trident-autosupport"
readonly __DEFAULT_TRIDENT_IMAGE_NAME="trident"
readonly __DEFAULT_CONNECTOR_OPERATOR_IMAGE_NAME="astra-connector-operator"
readonly __DEFAULT_CONNECTOR_IMAGE_NAME="astra-connector"
readonly __DEFAULT_NEPTUNE_IMAGE_NAME="controller"
readonly __DEFAULT_TRIDENT_ACP_IMAGE_NAME="trident-acp"

readonly __DEFAULT_CONNECTOR_NAMESPACE="astra-connector"
readonly __DEFAULT_TRIDENT_NAMESPACE="trident"

readonly __GENERATED_CRS_DIR="./astra-generated"
readonly __GENERATED_CRS_FILE="$__GENERATED_CRS_DIR/crs.yaml"
readonly __GENERATED_OPERATORS_DIR="$__GENERATED_CRS_DIR/operators"
readonly __GENERATED_KUSTOMIZATION_FILE="$__GENERATED_OPERATORS_DIR/kustomization.yaml"
readonly __GENERATED_PATCHES_TORC_FILE="$__GENERATED_CRS_DIR/post-deploy-patches_torc"
readonly __GENERATED_PATCHES_TRIDENT_OPERATOR_FILE="$__GENERATED_OPERATORS_DIR/post-deploy-patches_trident-operator"
readonly __GENERATED_PATCHES_RESOURCE_LIMITS="$__GENERATED_CRS_DIR/post-deploy-patches_resource_limits"

readonly __RESOURCE_LIMITS_SMALL="small"
readonly __RESOURCE_LIMITS_MEDIUM="medium"
readonly __RESOURCE_LIMITS_LARGE="large"
readonly __RESOURCE_LIMITS_CUSTOM="custom"
readonly __RESOURCE_LIMITS_SKIP="skip"
readonly __RESOURCE_LIMITS_VALID_PRESETS=("$__RESOURCE_LIMITS_SMALL" "$__RESOURCE_LIMITS_MEDIUM" \
    "$__RESOURCE_LIMITS_LARGE" "$__RESOURCE_LIMITS_CUSTOM" "$__RESOURCE_LIMITS_SKIP")

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
    LABELS="${LABELS:-}"
    # RESOURCE_LIMITS_PRESET will only be used if both RESOURCE_LIMITS_CUSTOM_CPU and RESOURCE_LIMITS_CUSTOM_MEMORY are empty.
    RESOURCE_LIMITS_PRESET="${RESOURCE_LIMITS_PRESET:-}"
    RESOURCE_LIMITS_CUSTOM_CPU="${RESOURCE_LIMITS_CUSTOM_CPU:-}" # Plain number
    RESOURCE_LIMITS_CUSTOM_MEMORY="${RESOURCE_LIMITS_CUSTOM_MEMORY:-}" # Plain number, assumed to be in 'Gi'

    # ------------ IMAGE REGISTRY ------------
    # The REGISTRY environment variables follow a hierarchy; each layer overwrites the previous, if specified.
    # Note: the registry should not include a repository path. For example, if an image is hosted at
    # `cr.astra.netapp.io/common/image/path/astra-connector`, then the registry should be set to
    # `cr.astra.netapp.io` and NOT `cr.astra.netapp.io/common/image/path`.
    IMAGE_REGISTRY="${IMAGE_REGISTRY}"
        DOCKER_HUB_IMAGE_REGISTRY="${DOCKER_HUB_IMAGE_REGISTRY:-${IMAGE_REGISTRY:-$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY}}"
            TRIDENT_OPERATOR_IMAGE_REGISTRY="${TRIDENT_OPERATOR_IMAGE_REGISTRY:-$DOCKER_HUB_IMAGE_REGISTRY}"
            TRIDENT_AUTOSUPPORT_IMAGE_REGISTRY="${TRIDENT_AUTOSUPPORT_IMAGE_REGISTRY:-$DOCKER_HUB_IMAGE_REGISTRY}"
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
            TRIDENT_AUTOSUPPORT_IMAGE_REPO="${TRIDENT_AUTOSUPPORT_IMAGE_REPO:-"$(join_rpath "$DOCKER_HUB_BASE_REPO" "$__DEFAULT_TRIDENT_AUTOSUPPORT_IMAGE_NAME")"}"
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
            TRIDENT_AUTOSUPPORT_IMAGE_TAG="${TRIDENT_AUTOSUPPORT_IMAGE_TAG:-$DOCKER_HUB_IMAGE_TAG}"
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

components_include_neptune() {
    components_include_connector
}

components_include_trident() {
    if str_contains_at_least_one "$COMPONENTS" \
            "$__COMPONENTS_ALL_ASTRA_CONTROL" "$__COMPONENTS_TRIDENT_AND_ACP" "$__COMPONENTS_TRIDENT_ONLY"; then
        return 0
    fi
    return 1
}

components_include_acp() {
    if str_contains_at_least_one "$COMPONENTS" \
            "$__COMPONENTS_ALL_ASTRA_CONTROL" "$__COMPONENTS_TRIDENT_AND_ACP" "$__COMPONENTS_ACP_ONLY"; then
        return 0
    fi
    return 1
}

get_trident_namespace() {
    echo "${_EXISTING_TRIDENT_NAMESPACE:-"${NAMESPACE:-"${__DEFAULT_TRIDENT_NAMESPACE}"}"}"
}

get_connector_operator_namespace() {
    echo "${NAMESPACE:-"${__DEFAULT_CONNECTOR_OPERATOR_NAMESPACE}"}"
}

get_connector_namespace() {
    echo "${NAMESPACE:-"${__DEFAULT_CONNECTOR_NAMESPACE}"}"
}

trident_is_missing() {
    [ -z "$_EXISTING_TRIDENT_NAMESPACE" ] && return 0
    return 1
}

should_skip_trident_upgrade() {
    [ "$SKIP_TRIDENT_UPGRADE" == "true" ] && return 0
    return 1
}

get_config_trident_image() {
    as_full_image "$TRIDENT_IMAGE_REGISTRY" "$TRIDENT_IMAGE_REPO" "$TRIDENT_IMAGE_TAG"
}

get_config_trident_operator_image() {
    as_full_image "$TRIDENT_OPERATOR_IMAGE_REGISTRY" "$TRIDENT_OPERATOR_IMAGE_REPO" "$TRIDENT_OPERATOR_IMAGE_TAG"
}

get_config_trident_autosupport_image() {
    as_full_image "$TRIDENT_AUTOSUPPORT_IMAGE_REGISTRY" "$TRIDENT_AUTOSUPPORT_IMAGE_REPO" "$TRIDENT_AUTOSUPPORT_IMAGE_TAG"
}

get_config_acp_image() {
    as_full_image "$TRIDENT_ACP_IMAGE_REGISTRY" "$TRIDENT_ACP_IMAGE_REPO" "$TRIDENT_ACP_IMAGE_TAG"
}


trident_image_needs_upgraded() {
    local -r configured_image="$(get_config_trident_image)"

    logdebug "Checking if Trident image needs upgraded: $_EXISTING_TRIDENT_IMAGE vs $configured_image"
    if [ "$_EXISTING_TRIDENT_IMAGE" != "$configured_image" ]; then
        return 0
    fi

    return 1
}

trident_operator_image_needs_upgraded() {
    local -r configured_image="$(get_config_trident_operator_image)"

    logdebug "Checking if Trident image needs upgraded: $_EXISTING_TRIDENT_OPERATOR_IMAGE vs $configured_image"
    if [ "$_EXISTING_TRIDENT_OPERATOR_IMAGE" != "$configured_image" ]; then
        return 0
    fi

    return 1
}

acp_image_needs_upgraded() {
    local -r configured_image="$(get_config_acp_image)"

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

    [ -z "${!registry_var}" ] && fatal "component '$component_name' invalid: variable '$registry_var' is empty"
    [ -z "${!repo_var}" ] && fatal "component '$component_name' invalid: variable '$repo_var' is empty"
    [ -z "${!tag_var}" ] && fatal "component '$component_name' invalid: variable '$tag_var' is empty"
    [ -z "${!default_image_name_var}" ] && fatal "component '$component_name' invalid: variable '$default_image_name_var' is empty"

    [ "$current_image" != "$default_image" ] && return 0
    return 1
}

config_trident_operator_image_is_custom() {
    if config_image_is_custom "TRIDENT_OPERATOR" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO"; then
        return 0
    fi
    return 1
}

config_trident_autosupport_image_is_custom() {
    if config_image_is_custom "TRIDENT_AUTOSUPPORT" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO"; then
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

get_preset_recommendation() {
    local -r node_count="$1"
    local -r namespace_count="$2"

    [ "$node_count" -lt 1 ] && fatal "invalid node_count '$node_count' given: must be a number greater than 0"
    [ "$namespace_count" -lt 1 ] && fatal "invalid namespace_count '$namespace_count' given: must be a number greater than 0"

    # TODO: these are placeholder recommendations
    if [ "$node_count" -gt 5 ] || [ "$namespace_count" -lt 25 ]; then
        echo "${__RESOURCE_LIMITS_SMALL}"
    elif [ "$node_count" -gt 15 ] || [ "$namespace_count" -lt 75 ]; then
        echo "${__RESOURCE_LIMITS_MEDIUM}"
    else
        echo "${__RESOURCE_LIMITS_LARGE}"
    fi
}

get_limits_for_preset() {
    local -r preset="$1"
    [ -z "$preset" ] && fatal "no preset given"

    # TODO: these are placeholders too
    if [ "$preset" == "$__RESOURCE_LIMITS_SMALL" ]; then
        echo '{"cpu": "1", "memory": "2Gi"}'
    elif [ "$preset" == "$__RESOURCE_LIMITS_MEDIUM" ]; then
        echo '{"cpu": "3", "memory": "6Gi"}'
    elif [ "$preset" == "$__RESOURCE_LIMITS_LARGE" ]; then
        echo '{"cpu": "6", "memory": "12Gi"}'
    elif [ "$preset" == "$__RESOURCE_LIMITS_CUSTOM" ]; then
        local -r custom_cpu="${RESOURCE_LIMITS_CUSTOM_CPU:-"(manual)"}"
        local custom_mem="${RESOURCE_LIMITS_CUSTOM_MEMORY:-"(manual)"}"
        [ -n "$RESOURCE_LIMITS_CUSTOM_MEMORY" ] && custom_mem+="Gi"
        echo '{"cpu": "'"$custom_cpu"'", "memory": "'"$custom_mem"'"}'
    elif [ "$preset" == "$__RESOURCE_LIMITS_SKIP" ]; then
        echo ""
    elif str_matches_at_least_one "$preset" "${__RESOURCE_LIMITS_VALID_PRESETS[@]}"; then
        fatal "preset '$preset' is apparently valid but this function hasn't been updated to take it into account"
    fi

    [ -z "$preset" ] && fatal "invalid preset '$preset' given"
}

get_limits_for_preset_fancy() {
    get_limits_for_preset "$1" | jq -r '"cpu: \(.cpu), memory: \(.memory)"'
}

get_limits_list_fancy() {
    local msg=""
    for preset in "${__RESOURCE_LIMITS_VALID_PRESETS[@]}" ; do
        msg+="$preset -- $(get_limits_for_preset_fancy "$preset")${__NEWLINE}"
    done
    echo "${msg%${__NEWLINE}}"
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

# prefix_dryrun will add a prefix to the given string when DRY_RUN=true.
prefix_dryrun() {
    if is_dry_run; then
        echo "(DRY_RUN=true) $1"
    else
        echo "$1"
    fi
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

append_lines_to_file() {
    local -r file="$1"
    local -ar lines=("${@:2}")

    [ -z "$file" ] && fatal "no file given"
    [ "${#lines[@]}" -eq 0 ] && fatal "no lines given"

    for line in "${lines[@]}"; do
        echo "$line"
    done >> "$file"
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

# prompt_user_select_one will prompt the user to select one item from a list, putting the chosen
# value in the variable of the given name. The only time this function returns an error code (1) is when
# prompts are disabled and the variable has an invalid value (empty is also considered invalid unless provided
# as an option).
prompt_user_select_one() {
    local -r variable_name="$1"
    local -ra valid_values=("${@:2}")
    [ -z "$variable_name" ] && fatal "no variable name given"
    [ "${#valid_values[@]}" -lt 1 ] && fatal "no valid values given"

    if [ -z "${!variable_name}" ]; then
        prompt_user "${variable_name}" "Please select one of (${valid_values[*]}): "
    fi

    while true; do
        if str_matches_at_least_one "${!variable_name}" "${valid_values[@]}"; then
            return 0
        elif prompts_disabled; then
            return 1
        fi
        prompt_user "${variable_name}" "$variable_name value '${!variable_name}' is invalid. Select one of (${valid_values[*]}): "
    done
}

# prompt_user_number_greater_than_zero will prompt the user for a number greater than zero, putting the
# value in the variable of the given name. The only time this function returns an error code (1) is when
# prompts are disabled and the variable has an invalid value (empty is also considered invalid).
prompt_user_number_greater_than_zero() {
    local -r variable_name="$1"
    local -r initial_prompt_msg="$2"
    [ -z "$variable_name" ] && fatal "no variable name given"

    if [ -z "${!variable_name}" ]; then
        prompt_user "${variable_name}" "${initial_prompt_msg% } "
    fi

    while true; do
        if [ "${!variable_name}" -gt 0 ] &> /dev/null; then
            return 0
        elif prompts_disabled; then
            return 1
        fi
        prompt_user "${variable_name}" "$variable_name value '${!variable_name}' is invalid. Please enter a number greater than 0: "
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

str_contains_at_least_one() {
    local -r str_to_check="$1"
    local -a keywords=("${@:2}")

    [ -z "$str_to_check" ] && fatal "no str_to_check provided"
    [ "${#keywords[@]}" -eq 0 ] && fatal "no keywords provided"

    for keyword in "${keywords[@]}" ; do
        if echo "$str_to_check" | grep -qi -- "$keyword"; then
            return 0
        fi
    done

    return 1
}

str_matches_at_least_one() {
    local -r str_to_check="$1"
    local -a keywords=("${@:2}")

    [ -z "$str_to_check" ] && fatal "no str_to_check provided"
    [ "${#keywords[@]}" -eq 0 ] && fatal "no keywords provided"

    for keyword in "${keywords[@]}" ; do
        if [ "$str_to_check" == "$keyword" ]; then
            return 0
        fi
    done

    return 1
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

apply_kubectl_patch() {
    local patch="$1"
    local dry_run_flag=""

    [ -z "$patch" ] && fatal "no patch provided"
    str_contains_at_least_one "$patch" "kubectl patch" && fatal "patch '$patch' should exclude 'kubectl patch'"
    is_dry_run && dry_run_flag=" --dry-run=client"

    local -r command="kubectl patch$dry_run_flag $patch"
    local result="$command: "
    if eval "$command" &> /dev/null; then
        logdebug "$result OK"
        return 0
    else
        logdebug "$result failed"
        return 1
    fi
}

apply_kubectl_patches() {
    local -a patches=("${@}")

    if [ "${#patches}" -gt 0 ]; then
        for p in "${patches[@]}"; do
            apply_kubectl_patch "$p"
        done
    else
        logdebug "no patches to apply"
    fi
}

wait_for_deployment_running() {
    local -r deployment="$1"
    local -r namespace="${2:-"default"}"
    local -r timeout="${3:-"2m"}"
    [ -z "$deployment" ] && fatal "no deployment name given"

    if kubectl rollout status -n "$namespace" "deploy/$deployment" --timeout="$timeout" &> /dev/null; then
        return 0
    fi
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

wait_for_resource_created() {
    local -r resource="$1"
    local -r namespace="$2"
    local -r timeout="${3:-60}"

    [ -z "$resource" ] && fatal "no resource given"
    [ -z "$namespace" ] && fatal "no namespace given"

    local max_checks=1
    if (( timeout > 0 )); then
        max_checks=$(( timeout / 5 ))
        if (( max_checks <= 0 )); then
            max_checks=1
        fi
    fi

    logdebug "waiting for resource '$resource' in namespace '$namespace' to be created (timeout=$timeout)"
    local counter=0
    while ((counter < max_checks)); do
        if kubectl get -n "$namespace" "$resource" &> /dev/null; then
            logdebug "resource '$resource' found"
            return 0
        else
            logdebug "resource '$resource' not yet created"
            ((counter++))
            sleep 5
        fi
    done

    return 1
}

get_container_count() {
    local -r resource="$1"
    local -r namespace="$2"

    [ -z "$resource" ] && fatal "no resource given"
    [ -z "$namespace" ] && fatal "no namespace given"
    if ! str_contains_at_least_one "$resource" "deploy" "daemonset" "replicaset"; then
        fatal "invalid resource kind in '$resource': only deployments, daemonsets, and replicasets supported"
    fi

    local path=".spec.template.spec.containers"
    local count=0
    count="$(kubectl get "$resource" -n "$namespace" -o json 2> /dev/null | jq -r "$path | length" 2> /dev/null)"

    if [ -n "$count" ] || (( count > 0 )); then
        echo "$count"
        return 0
    fi

    return 1
}

create_kubectl_patch_for_containers() {
    local -r resource="$1" # Format: '$kind/$resource_name'
    local -r namespace="$2"
    local -r resource_path="$3" # Relative to the root of the container spec, e.g. "resources/limits"
    local -r patch_value="$4" # 'value' field of the JSON patch, e.g. `{"cpu": "5", "memory": "2Gi"}`
    local -r kind="$(dirname "$resource")"

    [ -z "$resource" ] && fatal "no resource given"
    [ -z "$namespace" ] && fatal "no namespace given"
    [ -z "$resource_path" ] && fatal "no resource_path given"
    [ -z "$patch_value" ] && fatal "no patch_value given"
    if ! echo "$resource" | grep -q -- "/"; then fatal "invalid format for given resource '$resource': use 'kind/resource_name'"; fi
    if ! echo "$patch_value" | jq &> /dev/null; then fatal "given patch_value '$patch_value' is not valid JSON"; fi
    if [ "$kind" != "deploy" ] && [ "$kind" != "pod" ]; then fatal "unsupported kind '$kind' given"; fi

    local containers_list_path_slash="/spec/template/spec/containers"

    local container_count=0
    local wait_timeout="" # Empty value == use default
    local dry_run_warning=""
    if is_dry_run; then
        # If dry running, the resource won't be created (though it might already exist) so no point in waiting
        wait_timeout=0
        dry_run_warning='"WARNING": "DRYRUN-GENERATED-PATCH",'
    fi

    if wait_for_resource_created "$resource" "$namespace" "$wait_timeout"; then
        container_count="$(get_container_count "$resource" "$namespace")"
        logdebug "found $container_count containers"
    elif is_dry_run; then
        container_count=1
        logdebug "resource '$resource' in namespace '$namespace' not found, continuing anyway because this is a dry run"
    else
        fatal "resource '$resource' in namespace '$namespace' was never created"
    fi

    if [ -z "$container_count" ] || (( container_count <= 0 )); then
        fatal "failed to get container count from resource '$resource' in namespace '$namespace'"
    fi

    local json_patch="[${__NEWLINE}"
    local current
    for (( i=0; i<"$container_count"; i++ )); do
        current='{'$dry_run_warning'"op": "replace","path": "'
        current+=$containers_list_path_slash'/'$i'/'$resource_path'","value": '$patch_value'}'
        json_patch+="$(echo "$current" | jq -r),"
        if (( i < container_count-1 )); then
            json_patch+="${__NEWLINE}"
        fi
    done

    json_patch="${json_patch%,}" # Remove trailing comma
    json_patch+="]"

    # Using the global _return_value var instead of 'echo' otherwise we can't log anything else without
    # forcing the consumer of the function to use `tail -n 1` every time
    _return_value="$resource -n $namespace --type=json -p '$json_patch'"
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
    trident_vars+=("TRIDENT_AUTOSUPPORT_IMAGE_REGISTRY" "$TRIDENT_AUTOSUPPORT_IMAGE_REGISTRY")
    trident_vars+=("TRIDENT_IMAGE_REGISTRY" "$TRIDENT_IMAGE_REGISTRY")
    trident_vars+=("TRIDENT_OPERATOR_IMAGE_REPO" "$TRIDENT_OPERATOR_IMAGE_REPO")
    trident_vars+=("TRIDENT_AUTOSUPPORT_IMAGE_REPO" "$TRIDENT_AUTOSUPPORT_IMAGE_REPO")
    trident_vars+=("TRIDENT_IMAGE_REPO" "$TRIDENT_IMAGE_REPO")
    trident_vars+=("TRIDENT_OPERATOR_IMAGE_TAG" "$TRIDENT_OPERATOR_IMAGE_TAG")
    trident_vars+=("TRIDENT_AUTOSUPPORT_IMAGE_TAG" "$TRIDENT_AUTOSUPPORT_IMAGE_TAG")
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
        elif [ "$COMPONENTS" == "$__COMPONENTS_TRIDENT_AND_ACP" ]; then
            required_vars+=("${trident_vars[@]}")
            required_vars+=("${acp_vars[@]}")
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

        images_to_check+=("$TRIDENT_AUTOSUPPORT_IMAGE_REGISTRY" "$TRIDENT_AUTOSUPPORT_IMAGE_REPO" "$TRIDENT_AUTOSUPPORT_IMAGE_TAG")
        if config_trident_autosupport_image_is_custom; then images_to_check+=("$custom")
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

# step_query_user_for_resource_count is used to query the user on the number of nodes/namespaces they expect
# to have on their cluster, which will help us recommend the appropriate resource limit preset.
step_query_user_for_resource_count() {
    local -r resource="$1"
    if ! str_matches_at_least_one "$resource" "namespaces" "nodes"; then
        fatal "invalid resource '$resource': only 'namespaces' or 'nodes' supported"
    fi
    local -r manual_entry_msg="How many $resource on do you expect to have in this cluster?"
    local msg=""

    _count="$(kubectl get "$resource" -o json | jq -r '.items | length' 2> /dev/null)"
    if [ -z "$_count" ] || [ "$_count" -lt 1 ]; then
        _count=""
        msg="Failed to detect number of $resource.${__NEWLINE}$manual_entry_msg"
        if ! prompt_user_number_greater_than_zero _count "$msg"; then
            msg="Failed to detect number of $resource."
            msg+=" Please verify your connection to the cluster and try again,"
            msg+=" or set RESOURCE_LIMITS_PRESET to one of the following values:"
            msg+=" ${__RESOURCE_LIMITS_VALID_PRESETS[*]}"
            add_problem "$msg"
            return 1
        fi
    else
        msg="We detected $_count $resource on your cluster.${__NEWLINE}"
        msg+="Is this accurate ('no' to enter a number manually)?"
        if ! prompt_user_yes_no "$msg"; then
            _user_count=""
            if ! prompt_user_number_greater_than_zero _user_count "$manual_entry_msg"; then
                # This failure path should not be possible (because the user needs to have prompts enabled)
                # which is why it's a fatal error instead of a problem
                fatal "failed to get $resource count from user"
            fi
            _count="$_user_count"
        fi
    fi

    loginfo "Proceeding with a count of $_count $resource."
    _return_value="$_count"
}

step_determine_resource_limit_preset() {
    local valid_presets=("${__RESOURCE_LIMITS_VALID_PRESETS[@]}")
    local valid_presets_str="${__RESOURCE_LIMITS_VALID_PRESETS[*]}"
    logheader "$__INFO" "Resolving resource limit configuration..."

    if [ -n "$RESOURCE_LIMITS_CUSTOM_CPU" ] || [ -n "$RESOURCE_LIMITS_CUSTOM_MEMORY" ]; then
        RESOURCE_LIMITS_PRESET="$__RESOURCE_LIMITS_CUSTOM"
        loginfo "Custom resource limits have been set by user ($(get_limits_for_preset_fancy "$__RESOURCE_LIMITS_CUSTOM"))"
    elif [ -z "$RESOURCE_LIMITS_PRESET" ]; then
        step_query_user_for_resource_count "nodes"
        local -r node_count="$_return_value"

        step_query_user_for_resource_count "namespaces"
        local -r namespace_count="$_return_value"

        exit_if_problems

        RESOURCE_LIMITS_PRESET="$(get_preset_recommendation "$node_count" "$namespace_count")"
        if ! str_matches_at_least_one "$RESOURCE_LIMITS_PRESET" "${valid_presets[@]}"; then
            fatal "the recommended preset ($RESOURCE_LIMITS_PRESET) is invalid: valid values are ($valid_presets_str)"
        fi

        local -r preset_fancy="$(get_limits_for_preset_fancy "$RESOURCE_LIMITS_PRESET")"
        local msg="For $node_count nodes and $namespace_count namespaces,"
        msg+=" the recommended resource limit preset is: $RESOURCE_LIMITS_PRESET ($preset_fancy)${__NEWLINE}"
        msg+="Proceed with preset '$RESOURCE_LIMITS_PRESET' (choose no to enter a preset manually)?"
        if ! prompt_user_yes_no "$msg"; then
            msg="${__NEWLINE}Available presets:${__NEWLINE}"
            msg+="$(get_limits_list_fancy)${__NEWLINE}"
            loginfo "$msg"
            RESOURCE_LIMITS_PRESET=""
            prompt_user_select_one RESOURCE_LIMITS_PRESET "${valid_presets[@]}"
        fi
    fi
    exit_if_problems

    loginfo "Resource limit preset is '$RESOURCE_LIMITS_PRESET'"
    add_to_config_builder RESOURCE_LIMITS_PRESET
    if [ "$RESOURCE_LIMITS_PRESET" == "$__RESOURCE_LIMITS_CUSTOM" ]; then
        if ! prompt_user_number_greater_than_zero RESOURCE_LIMITS_CUSTOM_CPU "Enter a CPU limit:"; then
            add_problem "RESOURCE_LIMITS_CUSTOM_CPU value '$RESOURCE_LIMITS_CUSTOM_CPU' is invalid"
        fi
        # Would ideally allow the user to input the suffix themselves to allow greater flexibility,
        # but that would require more complex parsing of their input. Maybe later, if it proves to
        # be needed.
        if ! prompt_user_number_greater_than_zero RESOURCE_LIMITS_CUSTOM_MEMORY "Enter a memory limit (Gi):"; then
            add_problem "RESOURCE_LIMITS_CUSTOM_MEMORY value '$RESOURCE_LIMITS_CUSTOM_MEMORY' is invalid"
        fi
        add_to_config_builder RESOURCE_LIMITS_CUSTOM_CPU
        add_to_config_builder RESOURCE_LIMITS_CUSTOM_MEMORY
    elif [ "$RESOURCE_LIMITS_PRESET" == "$__RESOURCE_LIMITS_SKIP" ]; then
        return 0
    fi

    _PROCESSED_RESOURCE_LIMITS="$(get_limits_for_preset "$RESOURCE_LIMITS_PRESET")"
    loginfo "Proceeding with resource limits: $(get_limits_for_preset_fancy "$RESOURCE_LIMITS_PRESET")"
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

    local -r connector_namespace="$(get_connector_namespace)"
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
}

step_collect_existing_trident_info() {
    logheader $__INFO "Checking if Trident is installed..."

    # TORC CR definition
    local -r torc_crd="$(kubectl get crd tridentorchestrators.trident.netapp.io -o name 2> /dev/null)"
    if [ -z "$torc_crd" ]; then
        logdebug "tridentorchestrator crd: not found"
        loginfo "* Trident installation not found."
        return 0
    else
        logdebug "tridentorchestrator crd: OK"
    fi

    # TORC CR
    local -r torc_json="$(kubectl get tridentorchestrator -A -o jsonpath="{.items[0]}" 2> /dev/null)"
    if [ -z "$torc_json" ]; then
        logdebug "tridentorchestrator: not found"
        loginfo "* Trident installation not found."
        return 0
    else
        logdebug "tridentorchestrator: OK"
    fi

    # Trident namespace
    local -r trident_ns="$(echo "$torc_json" | jq -r '.spec.namespace')"
    if [ -z "$(kubectl get namespace "$trident_ns" -o name 2> /dev/null)" ]; then
        logdebug "trident namespace '$trident_ns': not found"
        loginfo "* Trident Orchestrator exists, but configured namespace '$trident_ns' not found on cluster."
        return 0
    else
        logdebug "trident namespace '$trident_ns': OK"
    fi
    _EXISTING_TORC_NAME="$(echo "$torc_json" | jq -r '.metadata.name')"
    _EXISTING_TRIDENT_NAMESPACE="$trident_ns"
    logdebug "trident orchestrator: $_EXISTING_TORC_NAME"
    logdebug "trident namespace: $trident_ns"

    # Trident image
    local -r trident_image="$(echo "$torc_json" | jq -r ".spec.tridentImage" 2> /dev/null)"
    if [ -n "$trident_image" ]; then
        _EXISTING_TRIDENT_IMAGE="$trident_image"
        logdebug "trident image: $trident_image"
    else
        logdebug "trident image: not found"
    fi

    # ACP enabled/disabled
    local -r acp_enabled="$(echo "$torc_json" | jq -r '.spec.enableACP' 2> /dev/null)"
    if [ "$acp_enabled" == "true" ]; then
        logdebug "trident ACP enabled: yes"
        _EXISTING_TRIDENT_ACP_ENABLED="true"
    else
        _EXISTING_TRIDENT_ACP_ENABLED="false"
        logdebug "trident ACP enabled: no"
    fi

    # ACP image
    local -r acp_image="$(echo "$torc_json" | jq -r '.spec.acpImage' 2> /dev/null)"
    if [ -n "$acp_image" ]; then
        logdebug "trident ACP image: $acp_image"
        _EXISTING_TRIDENT_ACP_IMAGE="$acp_image"
    else
        logdebug "trident ACP image: not found"
    fi

    # Trident operator
    local -r trident_operator_json="$(kubectl get deploy/trident-operator -n "$trident_ns" -o json 2> /dev/null)"
    if [ -n "$trident_operator_json" ]; then
        local -r containers_length="$(echo "$trident_operator_json" | jq -r '.spec.template.spec.containers | length' 2> /dev/null)"
        # Assume there's only one container (and it's the trident-operator), but if that ever changes,
        # we'll at least learn about it. TODO: make this more robust
        if [ -z "$containers_length" ] || [ "$containers_length" -ne 1 ]; then
            fatal "expected only one container in trident-operator deployment (found '$containers_length')"
        fi

        _EXISTING_TRIDENT_OPERATOR_IMAGE="$(echo "$trident_operator_json" | jq -r '.spec.template.spec.containers[0].image' 2> /dev/null)"
        if [ -z "$_EXISTING_TRIDENT_OPERATOR_IMAGE" ]; then
            fatal "failed to get the existing trident-operator image"
        fi
    else
        logdebug "trident operator: not found"
    fi
}

step_generate_trident_fresh_install_yaml() {
    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    local -r crs_file="$__GENERATED_CRS_FILE"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"
    [ ! -f "$crs_file" ] && touch "$crs_file"

    logheader $__DEBUG "Generating Trident YAML files..."

    # TODO ASTRACTL-32183: use _KUBERNETES_VERSION to choose the right bundle yaml (pre 1.25 or post 1.25)
    insert_into_file_after_pattern "$kustomization_file" "resources:" \
        "- https://github.com/NetApp-Polaris/astra-deploy/trident-installer/deploy?ref=$__GIT_REF_TRIDENT"
    logdebug "$kustomization_file: added resources entry for trident operator"

    local -r trident_image="$(get_config_trident_image)"
    local -r autosupport_image="$(get_config_trident_autosupport_image)"
    local -r acp_image="$(get_config_acp_image)"
    local -r namespace="$(get_trident_namespace)"
    local pull_secret="[]"
    local enable_acp="true"
    local labels_field_and_content=""
    if [ -n "$IMAGE_PULL_SECRET" ]; then pull_secret='["'$IMAGE_PULL_SECRET'"]'; fi
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
  autosupportImage: "${autosupport_image}"
  namespace: "${namespace}"
  tridentImage: "${trident_image}"
  imagePullSecrets: ${pull_secret}
  enableACP: ${enable_acp}
  acpImage: "${acp_image}"
---
EOF
    logdebug "$crs_file: added TridentOrchestrator CR"
}

step_generate_trident_operator_patch() {
    local -r namespace="${_EXISTING_TRIDENT_NAMESPACE}"
    local -r new_image="$(get_config_trident_operator_image)"
    [ -z "$namespace" ] && fatal "no existing trident namespace found"
    [ -z "$new_image" ] && fatal "no trident operator image found"

    logheader $__DEBUG "Generating Trident Operator patch"
    local -r patch='[{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"'"$new_image"'"}]'
    _PATCHES_TRIDENT_OPERATOR+=("deploy/trident-operator -n '$namespace' --type=json -p '$(echo "$patch" | jq)'")
}

step_generate_torc_patch() {
    local -r torc_name="$1"
    local -r trident_image="${2:-""}"
    local -r acp_image="${3:-""}"
    local -r enable_acp="${4:-""}"
    local -r autosupport_image="${5:-""}"
    if [ -z "$torc_name" ]; then fatal "no trident orchestrator name was given"; fi

    logheader $__DEBUG "Generating TORC patch"
    local torc_patch_list=""

    # Trident
    if [ -n "$trident_image" ]; then
        torc_patch_list+='{"op":"replace","path":"/spec/tridentImage","value":"'"$trident_image"'"},'
    fi

    # ACP
    if [ -n "$acp_image" ]; then
        torc_patch_list+='{"op":"replace","path":"/spec/acpImage","value":"'"$acp_image"'"},'
    fi
    if [ "$enable_acp" == "true" ]; then
        torc_patch_list+='{"op":"replace","path":"/spec/enableACP","value":true},'
    fi

    # Autosupport
    if [ -n "$autosupport_image" ]; then
        torc_patch_list+='{"op":"replace","path":"/spec/autosupportImage","value":"'"$autosupport_image"'"},'
    fi

    if [ "${#torc_patch_list[@]}" -gt 0 ]; then
        torc_patch_list="'$(echo "[${torc_patch_list%,}]" | jq)'"
        _PATCHES_TORC+=("tridentorchestrator $torc_name --type=json -p ${torc_patch_list}")
    fi
}

step_add_labels_to_kustomization() {
    local processed_labels="${1:-""}"
    local -r kustomization_file="${2}"

    [ -z "${processed_labels}" ] && return 0
    [ -z "${kustomization_file}" ] && fatal "no kustomization file given"
    [ ! -f "${kustomization_file}" ] && fatal "kustomization file '${kustomization_file}' does not exist"

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
    local -r operators_dir="$__GENERATED_OPERATORS_DIR"

    logheader "$__INFO" "$(prefix_dryrun "Applying resources...")"

    # Apply operator resources
    logdebug "apply operator resources"
    local output=""
    if ! is_dry_run; then
        output="$(kubectl apply -k "$operators_dir" 2>&1)"
        logdebug "kustomize apply output:${__NEWLINE}$output"
    fi
    loginfo "* Astra operators have been applied to the cluster."

    # Apply CRs (if we have any)
    if [ -f "$crs_file_path" ]; then
        logdebug "apply CRs"
        if grep -q "AstraConnector" $crs_file_path; then
            logdebug "delete previous astraconnect if it exists"
            if ! is_dry_run; then
                # Operator doesn't change the astraconnect spec automatically so we need to delete it first (if it exists)
                kubectl delete -n "$(get_connector_namespace)" deploy/astraconnect &> /dev/null
                output="$(kubectl apply -f "$crs_file_path")"
                logdebug "$output"
            fi
        fi
        loginfo "* Astra CRs have been applied to the cluster."
    else
        logdebug "No CRs file to apply"
    fi
}

step_apply_trident_operator_patches() {
    logheader "$__DEBUG" "$(prefix_dryrun "Applying Trident Operator patches...")"
    local -ra patches=("${_PATCHES_TRIDENT_OPERATOR[@]}")

    if [ "$LOG_LEVEL" == "$__DEBUG" ] && [ "${#patches[@]}" -gt 0 ]; then
        local disclaimer="# THIS FILE IS FOR DEBUGGING PURPOSES ONLY. These are the patches applied to the"
        disclaimer+="${__NEWLINE}# Trident Operator deployment when upgrading the Trident Operator."
        disclaimer+="${__NEWLINE}"
        append_lines_to_file "${__GENERATED_PATCHES_TRIDENT_OPERATOR_FILE}" "$disclaimer" "${patches[@]}"
    fi

    apply_kubectl_patches "${patches[@]}"
    logdebug "done"
}

step_apply_torc_patches() {
    logheader "$__DEBUG" "$(prefix_dryrun "Applying TORC patches...")"
    local -ra patches=("${_PATCHES_TORC[@]}")

    if [ "$LOG_LEVEL" == "$__DEBUG" ] && [ "${#patches[@]}" -gt 0 ]; then
        local disclaimer="# THIS FILE IS FOR DEBUGGING PURPOSES ONLY. These are the patches applied to the"
        disclaimer+="${__NEWLINE}# TridentOrchestrator resource when upgrading Trident or enabling ACP."
        disclaimer+="${__NEWLINE}"
        append_lines_to_file "${__GENERATED_PATCHES_TORC_FILE}" "$disclaimer" "${patches[@]}"
    fi

    apply_kubectl_patches "${patches[@]}"
    logdebug "done"
}

# step_generate_and_apply_resource_limit_patches generates and applies patches in one go, and should
# be called after we've applied all resources. This is because the content of the patches is dynamically
# generated based on the live resources, which itself is necessary for both the Neptune controller and the
# Astra connector since they are created by the operators, not by us. This does mean we could technically set
# the resource limits on the operators before we apply the resources, but whatever time we save is minimal
# compared to the extra complexity it would introduce due to having two separate processes.
step_generate_and_apply_resource_limit_patches() {
    [ "$RESOURCE_LIMITS_PRESET" == "$__RESOURCE_LIMITS_SKIP" ] && return 0
    [ -z "$_PROCESSED_RESOURCE_LIMITS" ] && fatal "_PROCESSED_RESOURCE_LIMITS is empty"

    local -r connector_ns="$(get_connector_namespace)"
    local -r trident_ns="$(get_trident_namespace)"
    local -r patch_path="resources"
    local -r patch_value="$_PROCESSED_RESOURCE_LIMITS"
    local -a patches_list_for_debugging=()

    logheader "$__DEBUG" "$(prefix_dryrun "Creating and applying resource-limit patches")"
    logdebug "configured limits: $patch_value"

    # Note 1: The order we patch these is somewhat important to minimize downtime, as the operators create resources
    # (such as the Neptune controller) after a delay. And so, operators get patched first, and operator-created
    # resources second, ideally in the order they are created.
    #
    # Note 2: Trident does not support setting resource limits as of this writing; the Trident Operator will clear out
    # any resource limits we set on the controller (even if it's post-deployment). That said, the Trident Operator
    # itself does "support" resource limits, in the sense that they don't get cleared out when we set them.

    # Trident Operator
    if ! components_include_trident; then
        logdebug "skipping trident operator resource limits"
    elif create_kubectl_patch_for_containers "deploy/trident-operator" "$trident_ns" "$patch_path" "$patch_value"; then
        patches_list_for_debugging+=("$_return_value")
        apply_kubectl_patch "$_return_value"
    else
        add_problem "Failed to create resource limit patch for the Trident Operator deployment (unknown error)"
    fi

    # Connector operator
    if ! components_include_connector; then
        logdebug "skipping connector operator resource limits"
    elif create_kubectl_patch_for_containers "deploy/operator-controller-manager" "$connector_ns" "$patch_path" "$patch_value"; then
        patches_list_for_debugging+=("$_return_value")
        apply_kubectl_patch "$_return_value"
    else
        add_problem "Failed to create resource limit patch for the Astra Connector deployment (unknown error)"
    fi
    exit_if_problems

    # Neptune controller
    if ! components_include_neptune; then
        logdebug "skipping neptune resource limits"
    elif create_kubectl_patch_for_containers "deploy/neptune-controller-manager" "$connector_ns" "$patch_path" "$patch_value"; then
        patches_list_for_debugging+=("$_return_value")
        apply_kubectl_patch "$_return_value"
    else
        add_problem "Failed to create resource limit patch for the Neptune deployment (unknown error)"
    fi

    # Connector
    if ! components_include_connector; then
        logdebug "skipping connector resource limits"
    elif create_kubectl_patch_for_containers "deploy/astraconnect" "$connector_ns" "$patch_path" "$patch_value"; then
        patches_list_for_debugging+=("$_return_value")
        apply_kubectl_patch "$_return_value"
    else
        add_problem "Failed to create resource limit patch for the Astra Connector deployment (unknown error)"
    fi

    if [ "$LOG_LEVEL" == "$__DEBUG" ] && [ "${#patches[@]}" -gt 0 ]; then
        local disclaimer="# THIS FILE IS FOR DEBUGGING PURPOSES ONLY. These are the patches applied to the"
        disclaimer+="${__NEWLINE}# Trident Operator deployment when upgrading the Trident Operator."
        disclaimer+="${__NEWLINE}"
        append_lines_to_file "${__GENERATED_PATCHES_RESOURCE_LIMITS}" "$disclaimer" "${patches[@]}"
    fi
    exit_if_problems

    logdebug "done"
}

step_monitor_deployment_progress() {
    local -r connector_operator_ns="$(get_connector_operator_namespace)"
    local -r connector_ns="$(get_connector_namespace)"
    local -r trident_ns="$(get_trident_namespace)"

    logheader "$__INFO" "$(prefix_dryrun "Monitoring deployment progress...")"
    if ! is_dry_run; then
        sleep 20 # Wait for initial resources to be created and operators to detect changes
    fi

    if components_include_connector; then
        if is_dry_run; then
            logdebug "skip monitoring connector components because it's a dry run"
        elif ! wait_for_deployment_running "operator-controller-manager" "$connector_operator_ns" "1m"; then
            add_problem "connector operator deploy: failed" "The Astra Connector Operator failed to deploy"
        elif ! wait_for_deployment_running "neptune-controller-manager" "$connector_ns" "3m"; then
            add_problem "neptune deploy: failed" "Neptune failed to deploy"
        elif ! wait_for_deployment_running "astraconnect" "$connector_ns" "3m"; then
            add_problem "astraconnect deploy: failed" "The Astra Connector failed to deploy"
        elif ! wait_for_cr_state "astraconnectors/astra-connector" ".status.natsSyncClient.status" "Registered with Astra" "$connector_ns"; then
            add_problem "cluster registration: failed" "Cluster registration failed"
        fi
    fi

    if components_include_trident || components_include_acp; then
        if is_dry_run; then
            logdebug "skip monitoring trident components because it's a dry run"
        elif ! wait_for_deployment_running "trident-operator" "$trident_ns" "1m"; then
            add_problem "trident operator: failed" "The Trident Operator failed to deploy"
        elif ! wait_for_deployment_running "trident-controller" "$trident_ns" "10m"; then
            add_problem "trident controller: failed" "Trident failed to deploy"
        fi
    fi
}

#======================================================================
#== Main
#======================================================================
logln $__INFO "====== Astra Cluster Installer v0.0.1 ======"
set_log_level
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
if components_include_connector && [ "$SKIP_ASTRA_CHECK" != "true" ]; then
    step_check_astra_control_reachable
    step_check_astra_cloud_and_cluster_id
else
    logdebug "skipping all Astra checks (COMPONENTS=${COMPONENTS}, SKIP_ASTRA_CHECK=${SKIP_ASTRA_CHECK})"
fi
exit_if_problems

# ------------ YAML GENERATION ------------
step_check_kubeconfig_choice
step_determine_resource_limit_preset
step_init_generated_dirs_and_files
step_kustomize_global_namespace_if_needed "$NAMESPACE" "$__GENERATED_KUSTOMIZATION_FILE"

# CONNECTOR yaml
if components_include_connector; then
    step_generate_astra_connector_yaml "$__GENERATED_CRS_DIR" "$__GENERATED_OPERATORS_DIR"
fi

# TRIDENT / ACP yaml
step_collect_existing_trident_info
if trident_is_missing; then
    step_generate_trident_fresh_install_yaml
elif [ -z "$_EXISTING_TRIDENT_OPERATOR_IMAGE" ]; then
    logwarn "Upgrading Trident without the Trident Operator is not currently supported, skipping."
else
    # Upgrade Trident/Operator?
    if components_include_trident && ! should_skip_trident_upgrade; then
        # Trident upgrade (includes operator upgrade if needed)
        if trident_image_needs_upgraded; then
            if config_trident_image_is_custom || prompt_user_yes_no "Would you like to upgrade Trident?"; then
                step_generate_torc_patch "$_EXISTING_TORC_NAME" "$(get_config_trident_image)" "" "" "$(get_config_trident_autosupport_image)"
                trident_operator_image_needs_upgraded && step_generate_trident_operator_patch
            else
                loginfo "Trident will not be upgraded."
            fi
        # Trident operator upgrade (standalone)
        elif trident_operator_image_needs_upgraded; then # TODO
            if config_trident_operator_image_is_custom || prompt_user_yes_no "Would you like to upgrade the Trident Operator?"; then
                step_generate_trident_operator_patch
            else
                loginfo "Trident Operator will not be upgraded."
            fi
        fi
    else
        logdebug "Skipping Trident upgrade (COMPONENTS=${COMPONENTS}, SKIP_TRIDENT_UPGRADE=${SKIP_TRIDENT_UPGRADE})"
    fi

    # Upgrade/Enable ACP?
    if components_include_acp; then
        # Enable ACP if needed (includes ACP upgrade)
        if ! acp_is_enabled; then
            if config_acp_image_is_custom || prompt_user_yes_no "Would you like to enable ACP?"; then
                step_generate_torc_patch "$_EXISTING_TORC_NAME" "" "$(get_config_acp_image)" "true"
            else
                loginfo "ACP will not be enabled."
            fi
        # ACP upgrade (ACP already enabled)
        elif acp_image_needs_upgraded; then
            if config_acp_image_is_custom || prompt_user_yes_no "Would you like to upgrade ACP?"; then
                step_generate_torc_patch "$_EXISTING_TORC_NAME" "" "$(get_config_acp_image)" "true"
            else
                loginfo "ACP will not be upgraded."
            fi
        fi
    else
        logdebug "Skipping ACP changes (COMPONENTS=${COMPONENTS})"
    fi

fi

# IMAGE REMAPS, LABELS, RESOURCE LIMITS yaml
step_add_labels_to_kustomization "${_PROCESSED_LABELS}" "${__GENERATED_KUSTOMIZATION_FILE}" "${__GENERATED_CRS_FILE}"
step_add_image_remaps_to_kustomization
exit_if_problems

# ------------ DEPLOYMENT ------------
step_apply_resources
step_apply_trident_operator_patches
step_apply_torc_patches
step_generate_and_apply_resource_limit_patches
step_monitor_deployment_progress
exit_if_problems

if ! is_dry_run; then
    logheader $__INFO "Cluster management complete!"
else
    [ "$LOG_LEVEL" == "$__DEBUG" ] && print_built_config
    logheader $__INFO "$(prefix_dryrun "See generated files")"
    loginfo "$(find "$__GENERATED_CRS_DIR" -type f)"
    _msg="You can run 'kustomize build $__GENERATED_OPERATORS_DIR > $__GENERATED_OPERATORS_DIR/resources.yaml'"
    _msg+=" to view the CRDs and operator resources that will be applied."
    logln $__INFO "$_msg"
    exit 0
fi

