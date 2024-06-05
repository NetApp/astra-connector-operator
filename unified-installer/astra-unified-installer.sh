#!/bin/bash

# -- Resources
# Deploy Trident Operator: https://docs.netapp.com/us-en/trident/trident-get-started/kubernetes-deploy-operator.html
# Upgrade Trident: https://docs.netapp.com/us-en/trident/trident-managing-k8s/upgrade-trident.html

#----------------------------------------------------------------------
#-- Private vars/constants
#----------------------------------------------------------------------

# ------------ STATEFUL VARIABLES ------------
_PROBLEMS=() # Any failed checks will be added to this array and the program will exit at specific checkpoints if not empty
_CONFIG_BUILDER=() # Contains the fully resolved config to be output at the end during a dry run
_KUBERNETES_VERSION=""

# _TRIDENT_COLLECTION_STEP_CALLED is used as a guardrail to prevent certain functions from being called before
# existing Trident information (if any) has been collected.
_TRIDENT_COLLECTION_STEP_CALLED="false"
_EXISTING_TORC_NAME="" # TORC is short for TridentOrchestrator (works with kubectl too)
_EXISTING_TRIDENT_NAMESPACE=""
_EXISTING_TRIDENT_IMAGE=""
_EXISTING_TRIDENT_VERSION=""
_EXISTING_TRIDENT_ACP_ENABLED=""
_EXISTING_TRIDENT_ACP_IMAGE=""
_EXISTING_TRIDENT_OPERATOR_IMAGE=""

# _PATCHES_ variables contain the k8s patches that will be applied after we've applied all CRs and kustomize resources.
# Entries should omit the 'kubectl patch' from the command, e.g. `deploy/astraconnect -n astra --type=json -p '[...]'`
_PATCHES_TORC=() # Patches for the TridentOrchestrator
_PATCHES_TRIDENT_OPERATOR=() # Patches for the Trident Operator

# _PROCESSED_LABELS_WITH_DEFAULT will contain an already indented, YAML-compliant "map" (in string form) of the given LABELS.
# Example: "    label1: value1\n    label2: value2\n    label3: value3" plus app.kubernetes.io/created-by: astra-unified-installer
_PROCESSED_LABELS_WITH_DEFAULT=""

# _PROCESSED_LABELS will contain an already indented, YAML-compliant "map" (in string form) of the given LABELS.
# Example: "    label1: value1\n    label2: value2\n    label3: value3"
_PROCESSED_LABELS=""

# ------------ CONSTANTS ------------
readonly __RELEASE_VERSION="24.02"
readonly __TRIDENT_VERSION="${__TRIDENT_VERSION_OVERRIDE:-"$__RELEASE_VERSION"}"

readonly -a __REQUIRED_TOOLS=("git" "jq" "kubectl" "curl" "grep" "sort" "uniq" "find" "base64" "wc" "awk")

# __GIT_REF_CONNECTOR_OPERATOR is set via github Actions when added to the Git Release
readonly __GIT_REF_CONNECTOR_OPERATOR="main" # Determines the ACOP branch from which the kustomize resources will be pulled
# TODO point to stable/24.06 when branch and image is available
readonly __GIT_REF_TRIDENT="TmpTrident24.02" # Determines the Trident branch from which the kustomize resources will be pulled

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
readonly __DEFAULT_TORC_NAME="trident"

readonly __PRODUCTION_AUTOSUPPORT_URL="https://support.netapp.com/put/AsupPut"

readonly __GENERATED_CRS_DIR="./astra-generated"
readonly __GENERATED_CRS_FILE="$__GENERATED_CRS_DIR/crs.yaml"
readonly __GENERATED_OPERATORS_DIR="$__GENERATED_CRS_DIR/operators"
readonly __GENERATED_KUSTOMIZATION_FILE="$__GENERATED_OPERATORS_DIR/kustomization.yaml"
readonly __GENERATED_PATCHES_TORC_FILE="$__GENERATED_CRS_DIR/post-deploy-patches_torc"
readonly __GENERATED_PATCHES_TRIDENT_OPERATOR_FILE="$__GENERATED_OPERATORS_DIR/post-deploy-patches_trident-operator"

readonly __DEBUG=10
readonly __INFO=20
readonly __WARN=30
readonly __ERROR=40
readonly __FATAL=50

# __ERR_FILE should be used when wanting to capture stdout and stderr output of a command separately.
# You can then use `get_captured_err` to get the captured error. Example:
#     captured_stdout="$(curl -sS https://bad-url.com 2> "$_ERR_FILE")"
#     captured_stderr="$(get_captured_err)"
readonly __ERR_FILE="tmp_last_captured_error.txt"
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
    # As a guardrail, setting DISABLE_PROMPTS=true will require the DO_NOT_MODIFY_EXISTING_TRIDENT env var to also be set.
    DISABLE_PROMPTS="${DISABLE_PROMPTS:-"false"}"
    DO_NOT_MODIFY_EXISTING_TRIDENT="${DO_NOT_MODIFY_EXISTING_TRIDENT}" # Required if DISABLED_PROMPTS=true

    # ------------ GENERAL ------------
    KUBECONFIG="${KUBECONFIG}"
    COMPONENTS="${COMPONENTS:-$__COMPONENTS_ALL_ASTRA_CONTROL}" # Determines what we'll install/upgrade
    IMAGE_PULL_SECRET="${IMAGE_PULL_SECRET:-}" # TODO ASTRACTL-32772: skip prompt if IMAGE_REGISTRY is default
    NAMESPACE="${NAMESPACE:-}" # Overrides EVERY resource's namespace (for fresh installs only, not upgrades)
    LABELS="${LABELS:-}"
    # SKIP_TLS_VALIDATION will skip TLS validation for all requests made during the script.
    SKIP_TLS_VALIDATION="${SKIP_TLS_VALIDATION:-"false"}"

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
    # Docker TAG environment variables
    # __DEFAULT_CONNECTOR_OPERATOR_IMAGE_TAG is set via Github Actions before adding this script to the Git Release
    readonly __DEFAULT_CONNECTOR_OPERATOR_IMAGE_TAG=""

    TRIDENT_IMAGE_TAG="${TRIDENT_IMAGE_TAG:-$__TRIDENT_VERSION}"
        TRIDENT_OPERATOR_IMAGE_TAG="${TRIDENT_OPERATOR_IMAGE_TAG:-$TRIDENT_IMAGE_TAG}"
        TRIDENT_AUTOSUPPORT_IMAGE_TAG="${TRIDENT_AUTOSUPPORT_IMAGE_TAG:-$TRIDENT_IMAGE_TAG}"
        TRIDENT_ACP_IMAGE_TAG="${TRIDENT_ACP_IMAGE_TAG:-$TRIDENT_IMAGE_TAG}"
    CONNECTOR_OPERATOR_IMAGE_TAG="${CONNECTOR_OPERATOR_IMAGE_TAG:-$__DEFAULT_CONNECTOR_OPERATOR_IMAGE_TAG}"



    # ------------ ASTRA CONNECTOR ------------
    ASTRA_CONTROL_URL="${ASTRA_CONTROL_URL:-"astra.netapp.io"}"
    ASTRA_CONTROL_URL="$(process_url "$ASTRA_CONTROL_URL" "https://")"
    ASTRA_API_TOKEN="${ASTRA_API_TOKEN}"
    ASTRA_ACCOUNT_ID="${ASTRA_ACCOUNT_ID}"
    ASTRA_CLOUD_ID="${ASTRA_CLOUD_ID}"
    ASTRA_CLUSTER_ID="${ASTRA_CLUSTER_ID}"
    CONNECTOR_HOST_ALIAS_IP="${CONNECTOR_HOST_ALIAS_IP:-""}"
    CONNECTOR_HOST_ALIAS_IP="$(process_url "$CONNECTOR_HOST_ALIAS_IP")"
    CONNECTOR_SKIP_TLS_VALIDATION="${CONNECTOR_SKIP_TLS_VALIDATION:-"${SKIP_TLS_VALIDATION:-"false"}"}"
    CONNECTOR_AUTOSUPPORT_ENROLLED="${CONNECTOR_AUTOSUPPORT_ENROLLED:-"false"}"
    CONNECTOR_AUTOSUPPORT_URL="${CONNECTOR_AUTOSUPPORT_URL:-$__PRODUCTION_AUTOSUPPORT_URL}"
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
    local api_token=$ASTRA_API_TOKEN
    local token_warning="We detected that your ASTRA_API_TOKEN was provided through the CONFIG_FILE,"
    token_warning+=" which may pose a security risk! Make sure to store the configuration file in a secure location,"
    token_warning+=" or consider moving the API token out of the file and providing it through the command line only when needed."
    if [ -z "$config_file" ]; then return 0; fi
    if [ ! -f "$config_file" ]; then
        add_problem "CONFIG_FILE '$config_file' does not exist" "Given CONFIG_FILE '$config_file' does not exist"
        return 1
    fi

    # shellcheck disable=SC1090
    source "$config_file"
    set_log_level

    # check if api token was populated after sourcing config file
    if [ "$api_token" != "$ASTRA_API_TOKEN" ]; then
        logwarn "$token_warning"
    fi

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

existing_trident_can_be_modified() {
    [ "$DO_NOT_MODIFY_EXISTING_TRIDENT" == "true" ] && return 1
    return 0
}

existing_trident_needs_modifications() {
    if [ "$_TRIDENT_COLLECTION_STEP_CALLED" != "true" ]; then
        fatal "this function should not be called until existing Trident information has been collected"
    fi
    trident_is_missing && return 1

    components_include_trident && trident_image_needs_upgraded && return 0
    components_include_trident && trident_operator_image_needs_upgraded && return 0
    components_include_acp && acp_image_needs_upgraded && return 0
    components_include_acp && ! acp_is_enabled && return 0

    return 1
}

trident_is_missing() {
    if [ "$_TRIDENT_COLLECTION_STEP_CALLED" != "true" ]; then
        fatal "this function should not be called until existing Trident information has been collected"
    fi
    [ -z "$_EXISTING_TRIDENT_NAMESPACE" ] && return 0
    return 1
}


trident_will_be_installed_or_modified() {
    if [ "$_TRIDENT_COLLECTION_STEP_CALLED" != "true" ]; then
        fatal "this function should not be called until existing Trident information has been collected"
    fi
    if trident_is_missing; then return 0; fi
    if existing_trident_needs_modifications && existing_trident_can_be_modified; then return 0; fi
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
    local -r default_tag="$4"
    if [ $# -ne 4 ]; then
      echo "config_image_is_custom() expects 4 arguments, but received $#."
      return 1
    fi

    [ -z "$component_name" ] && fatal "no component_name given"

    local -r registry_var="${component_name}_IMAGE_REGISTRY"
    local -r repo_var="${component_name}_IMAGE_REPO"
    local -r tag_var="${component_name}_IMAGE_TAG"
    local -r current_image="$(as_full_image "${!registry_var}" "${!repo_var}" "${!tag_var}")"

    local -r default_image_name_var="__DEFAULT_${component_name}_IMAGE_NAME"
    local -r default_repo="$(join_rpath "$default_base_repo" "${!default_image_name_var}")"
    local -r default_image="$(as_full_image "$default_registry" "$default_repo" "$default_tag")"

    [ -z "${!registry_var}" ] && fatal "component '$component_name' invalid: variable '$registry_var' is empty"
    [ -z "${!repo_var}" ] && fatal "component '$component_name' invalid: variable '$repo_var' is empty"
    [ -z "${!tag_var}" ] && fatal "component '$component_name' invalid: variable '$tag_var' is empty"
    [ -z "${!default_image_name_var}" ] && fatal "component '$component_name' invalid: variable '$default_image_name_var' is empty"

    [ "$current_image" != "$default_image" ] && return 0
    return 1
}

config_trident_operator_image_is_custom() {
    if config_image_is_custom "TRIDENT_OPERATOR" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO" "$__TRIDENT_VERSION"; then
        return 0
    fi
    return 1
}

config_trident_autosupport_image_is_custom() {
    if config_image_is_custom "TRIDENT_AUTOSUPPORT" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO" "$__TRIDENT_VERSION"; then
        return 0
    fi
    return 1
}

config_trident_image_is_custom() {
    if config_image_is_custom "TRIDENT" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO" "$__TRIDENT_VERSION"; then
        return 0
    fi
    return 1
}

config_connector_operator_image_is_custom() {
    if config_image_is_custom "CONNECTOR_OPERATOR" "$__DEFAULT_DOCKER_HUB_IMAGE_REGISTRY" "$__DEFAULT_DOCKER_HUB_IMAGE_BASE_REPO" "$__TRIDENT_VERSION"; then
        return 0
    fi
    return 1
}

config_connector_image_is_custom() {
    # CONNECTOR_IMAGE_TAG is optional, if the user set this consider it custom
    if [ -n "$CONNECTOR_IMAGE_TAG" ]; then
        return 0
    fi
    return 1
}

config_neptune_image_is_custom() {
    # NEPTUNE_IMAGE_TAG is optional, if the user set this consider it custom
    if [ -n "$NEPTUNE_IMAGE_TAG" ]; then
        return 0
    fi
    return 1
}

config_acp_image_is_custom() {
    if config_image_is_custom "TRIDENT_ACP" "$__DEFAULT_ASTRA_IMAGE_REGISTRY" "$__DEFAULT_ASTRA_IMAGE_BASE_REPO" "$__TRIDENT_VERSION"; then
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
get_captured_err() {
    if [ -f "$__ERR_FILE" ]; then
        cat "$__ERR_FILE"
        rm -f "$__ERR_FILE" &> /dev/null
    else
        echo ""
    fi
}

debug_is_on() {
    [ "$LOG_LEVEL" == "$__DEBUG" ] && return 0
    [ "$LOG_LEVEL" -lt "$__DEBUG" ] &> /dev/null && return 0
    return 1
}

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
    log_at_level $__WARN "WARNING: $1"
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

    step_cleanup_tmp_files
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
        read -p "${prompt_msg% } " -r "${var_name?}"
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

# process_url removes trailing slashes from the given url and sets the protocol to the one given
process_url() {
    local url="$1"
    local -r protocol="${2:-""}"
    [ -z "$url" ] && return 0

    url="${url#http://}" # Remove 'http://'
    url="${url#https://}" # Remove 'https://'
    url="${url%/}" # Remove trailing slash
    url="${protocol}${url}"

    echo "$url"
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

    if command -v "$tool" &>/dev/null; then
        return 0
    fi

    return 1
}

version_in_range() {
    local -r current=$1
    local -r min=$2
    local -r max=$3

    if [ -z "$current" ]; then fatal "no current version given"; fi
    if [ -z "$min" ]; then fatal "no min version given"; fi
    if [ -z "$max" ]; then fatal "no max version given"; fi
    if [ "${current%.0}" == "${max%.0}" ]; then return 0; fi

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

status_code_msg() {
    local -r status_code="$1"
    [ -z "$status_code" ] && fatal "no status code given"

    local status=""
    case "$status_code" in
      200) status="OK" ;;
      201) status="created" ;;
      204) status="no content" ;;
      400) status="bad request" ;;
      401) status="unauthorized" ;;
      403) status="forbidden" ;;
      404) status="not found" ;;
      500) status="internal server error" ;;
      503) status="service unavailable" ;;
      *) status="unknown" ;;
    esac
    echo "$status"
}

add_tls_validation_hint_to_err_if_needed() {
    local error_msg="$1"
    [ -z "$error_msg" ] && return 0

    local match_phrase="curl: (60) SSL certificate problem"
    if echo "$error_msg" | grep -qi "$match_phrase"; then
        error_msg="$match_phrase -- try setting SKIP_TLS_VALIDATION=true (WARNING: not for production use!)"
    fi

    echo "$error_msg"
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
    if [ "$SKIP_TLS_VALIDATION" == "true" ]; then
        skip_tls_validation_opt="-k"
    fi

    logdebug "$method --> '$url'"
    local -r result="$(curl -X "$method" -sS $skip_tls_validation_opt -w "\n%{http_code}" "$url" "${headers[@]}" 2> "$__ERR_FILE")"
    _return_error="$(add_tls_validation_hint_to_err_if_needed "$(get_captured_err)")"
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

    local -a args=('-sS' '-w' "\n%{http_code}")
    if [ -n "$encoded_creds" ]; then
        args+=("-H" "Authorization: Basic $encoded_creds")
    fi
    if [ "$SKIP_TLS_VALIDATION" == "true" ]; then
        args+=("-k")
    fi
    # We accept all formats via '*/*' because we only really care about the status code, but certain multi-platform
    # images require a more specific format and will return 404 if we only use '*/*', so we add those formats as well
    local accept_formats="*/*"
    accept_formats+=", application/vnd.docker.distribution.manifest.list.v1+json"
    accept_formats+=", application/vnd.docker.distribution.manifest.list.v2+json"
    accept_formats+=", application/vnd.oci.image.index.v1+json" # Required for ACP
    args+=("-H" "Accept: $accept_formats")

    local -r result="$(curl -X GET "${args[@]}" "https://$registry/v2/$image_repo/manifests/$image_tag" 2> "$__ERR_FILE")"
    local -r curl_err="$(get_captured_err)"
    local -r line_count="$(echo "$result" | wc -l)"
    _return_body="$(echo "$result" | head -n "$((line_count-1))")"
    _return_status="$(echo "$result" | tail -n 1)"
    _return_error=""

    if [ "$_return_status" == 200 ]; then
        return 0
    elif [ "$_return_status" == 404 ]; then
        _return_error="the image was not found"
    elif [ "$_return_status" != 000 ]; then
        _return_error="$(echo "$_return_body" | jq -r '.errors.[0].message' 2> /dev/null)"
        if [ -z "$_return_error" ] || [ "$_return_error" == "null" ]; then
            _return_error="$(status_code_msg "$_return_status")"
        fi
    else
        if [ -n "$curl_err" ]; then _return_error="$(add_tls_validation_hint_to_err_if_needed "$curl_err")"
        else _return_error="unknown error"; fi
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
    local -r timeout="${3:-"2"}" # Minutes
    [ -z "$deployment" ] && fatal "no deployment name given"

    local -r sleep_time="5" # Seconds
    local -r max_checks="$(( (timeout * 60) / sleep_time ))"
    local counter=0
    loginfo "Waiting on deployment/$deployment (timeout: $timeout)..."
    while ((counter < max_checks)); do
        if kubectl rollout status -n "$namespace" "deploy/$deployment" -w=false &> /dev/null; then
            logdebug "deploy/$deployment is now running"
            return 0
        else
            logdebug "waiting for deploy/$deployment to be running"
            ((counter++))
            sleep "$sleep_time"
        fi
    done

    return 1
}

wait_for_cr_state() {
    local -r resource="$1"
    local -r path="$2"
    local -r desired_state="$3"
    local -r namespace="${4:-"default"}"
    local -r timeout="${5:-"2"}" # Minutes
    [ -z "$resource" ] && fatal "no resource given"
    [ -z "$path" ] && fatal "no JSON path given"
    [ -z "$desired_state" ] && fatal "no desired state given"

    local -r sleep_time="5" # Seconds
    local -r max_checks="$(( (timeout * 60) / sleep_time ))"
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
            sleep "$sleep_time"
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

backup_kubernetes_resource() {
    local -r kind="$1"
    local -r resource_name="$2"
    local -r directory="$3"
    local -r namespace="${4:-""}"

    [ -z "$kind" ] && fatal "no kind given"
    [ -z "$resource_name" ] && fatal "no resource_name given"
    [ -z "$directory" ] && fatal "no directory given"
    ! [ -d "$directory" ] && fatal "directory '$directory' does not exist"

    local namespace_arg=""
    local backup_name="BACKUP_"
    if [ -n "$namespace" ]; then
        namespace_arg="--namespace='$namespace'"
        backup_name+="${namespace}_"
    fi
    backup_name+="${kind}_${resource_name}.yaml"

    local -r resource_name_yaml="$(kubectl get "$kind" "$resource_name" "$namespace_arg" -o yaml 2> $__ERR_FILE)"
    [ -z "$resource_name_yaml" ] && return 1
    _return_error="$(get_captured_err)"

    echo "$resource_name_yaml" > "${directory}/${backup_name}"
    echo "$backup_name"
}

exit_if_problems() {
    if [ ${#_PROBLEMS[@]} -ne 0 ]; then
        debug_is_on && print_built_config
        logheader $__ERROR "Cluster management failed! We've identified the following issues:"
        for err in "${_PROBLEMS[@]}"; do
            logerror "* $err"
        done
        step_cleanup_tmp_files
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
    elif config_has_at_least_one_custom_registry_or_repo; then
        local custom_reg_warning="We detected one or more custom registry or repo values"
        custom_reg_warning+=", but no IMAGE_PULL_SECRET was specified. If any of your images are hosted in a private"
        custom_reg_warning+=" registry, an image pull secret will need to be created and IMAGE_PULL_SECRET set."
        if prompts_disabled; then
            logwarn "$custom_reg_warning"
        elif prompt_user_yes_no "$custom_reg_warning${__NEWLINE}Would you like to specify a pull secret now?"; then
            prompt_user "IMAGE_PULL_SECRET" "Enter a value for IMAGE_PULL_SECRET: "
        fi
    fi
    add_to_config_builder "IMAGE_PULL_SECRET"
    add_to_config_builder "NAMESPACE"

    if prompts_disabled; then
        if [ -z "$DO_NOT_MODIFY_EXISTING_TRIDENT" ]; then
            local -r longer_msg="DO_NOT_MODIFY_EXISTING_TRIDENT is required when prompts are disabled."
            add_problem "DO_NOT_MODIFY_EXISTING_TRIDENT: required (prompts disabled)" "$longer_msg"
        fi
    fi
    add_to_config_builder "DISABLE_PROMPTS"
    add_to_config_builder "DO_NOT_MODIFY_EXISTING_TRIDENT"

    # Fully optional env vars
    add_to_config_builder "SKIP_TLS_VALIDATION"
    add_to_config_builder "CONNECTOR_SKIP_TLS_VALIDATION"
    add_to_config_builder "CONNECTOR_HOST_ALIAS_IP"
    add_to_config_builder "CONNECTOR_AUTOSUPPORT_ENROLLED"
    add_to_config_builder "CONNECTOR_AUTOSUPPORT_URL"

    # Add our default labels
    local -r label_indent="    "
    local -a default_labels=("app.kubernetes.io/created-by=astra-unified-installer")
    _PROCESSED_LABELS_WITH_DEFAULT="$(process_labels_to_yaml "${default_labels[*]}" "$label_indent")"

    # Add user's custom labels
    if [ -n "${LABELS}" ]; then
        _PROCESSED_LABELS_WITH_DEFAULT+="${__NEWLINE}$(process_labels_to_yaml "${LABELS}" "$label_indent")"
        if [ -z "${_PROCESSED_LABELS_WITH_DEFAULT}" ]; then
            add_problem "label processing: failed" "The given LABELS could not be parsed."
        fi
    fi
    add_to_config_builder "LABELS"

     # Add user's custom labels
        if [ -n "${LABELS}" ]; then
            _PROCESSED_LABELS+="$(process_labels_to_yaml "${LABELS}" "$label_indent")"
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

step_check_volumesnapshotclasses() {
    logheader $__INFO "Checking for volumesnapshotclasses crd..."

    local -r volsnapclass_crd="$(kubectl get crd volumesnapshotclasses.snapshot.storage.k8s.io -o name 2> /dev/null)"
    if [ -z "$volsnapclass_crd" ]; then
        logwarn "We didn't find the volumesnapshotclasses CRD on the cluster! Installation will proceed, but snapshots will not work until this is corrected."
    else
        logdebug "volumesnapshotclasses crd: OK"
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
    # It might be possible to run the test and let the pod crash, after which we examine the pod's events or
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


        if config_connector_image_is_custom; then
          # Only check connector image if user has overridden the connector-operator's default.
          # We do not know the default version and cannot check it due to it being hard coded within the connector-operator image.
          images_to_check+=("$CONNECTOR_IMAGE_REGISTRY" "$CONNECTOR_IMAGE_REPO" "$CONNECTOR_IMAGE_TAG" "$custom")
        else
          # Get the default connector tag
          local file_content
          file_content=$(curl -sS "https://raw.githubusercontent.com/NetApp/astra-connector-operator/$CONNECTOR_OPERATOR_IMAGE_TAG/common/connector_version.txt")
          # Trim new lines and white space
          local -r connector_tag="${file_content//[[:space:]]/}"
          if [ -z "$connector_tag" ]; then
             logwarn "Cannot guarantee the existence of the Connector image due to a failure in resolving the default image tag, skipping check"
          else
            images_to_check+=("$CONNECTOR_IMAGE_REGISTRY" "$CONNECTOR_IMAGE_REPO" "$connector_tag" "$default")
          fi
        fi

        if config_neptune_image_is_custom; then
          # Only check neptune image if user has overridden the connector-operator's default.
          # We do not know the default version and cannot check it due to it being hard coded within the connector-operator image.
          images_to_check+=("$NEPTUNE_IMAGE_REGISTRY" "$NEPTUNE_IMAGE_REPO" "$NEPTUNE_IMAGE_TAG" "$custom")
        else
          # Get the default connector tag
          local file_content
          file_content=$(curl -sS "https://raw.githubusercontent.com/NetApp/astra-connector-operator/$CONNECTOR_OPERATOR_IMAGE_TAG/common/neptune_manager_tag.txt")
          # Trim new lines and white space
          local -r neptune_tag="${file_content//[[:space:]]/}"
          if [ -z "$neptune_tag" ]; then
             logwarn "Cannot guarantee the existence of the Neptune image due to a failure in resolving the default image tag, skipping check"
          else
            images_to_check+=("$NEPTUNE_IMAGE_REGISTRY" "$NEPTUNE_IMAGE_REPO" "$neptune_tag" "$default")
          fi
        fi
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
            local problem="[$full_image] image check failed"
            logdebug "body: '$body'"

            if is_docker_hub "$registry" && [ "$status" != 200 ] && [ "$status" != 404 ]; then
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
                logwarn "Cannot guarantee the existence of custom Docker Hub image '$full_image', skipping."
            elif [ "$status" != 200 ] || [ -n "$error" ]; then
                add_problem "$problem: $error ($status)"
            fi
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
    local -r status="$_return_status"
    local -r body="$_return_body"
    local err="$_return_error"
    if [ "$status" == 200 ]; then
        logdebug "astra control: OK"
    else
        if [ "$status" != 000 ] || [ -z "$err" ]; then err="$(status_code_msg "$status")"; fi
        logdebug "body: '$body'"
        add_problem "Failed to contact Astra Control at url '$ASTRA_CONTROL_URL': $err ($status)"
        return 1
    fi
}

step_check_astra_cloud_and_cluster_id() {
    make_astra_control_request "/topology/v1/clouds/$ASTRA_CLOUD_ID"
    local status="$_return_status"
    local body="$_return_body"
    local err="$_return_error"
    if [ "$status" == 200 ]; then
        logdebug "astra control cloud_id: OK"
    else
        if [ "$status" != 000 ] || [ -z "$err" ]; then err="$(status_code_msg "$status")"; fi
        logdebug "body: '$body'"
        add_problem "Given ASTRA_CLOUD_ID did not pass validation: $err ($status)"
        return 1
    fi

    make_astra_control_request "/topology/v1/clouds/$ASTRA_CLOUD_ID/clusters/$ASTRA_CLUSTER_ID"
    status="$_return_status"
    body="$_return_body"
    err="$_return_error"
    if [ "$status" == 200 ]; then
        logdebug "astra control cluster_id: OK"
    else
        if [ "$status" != 000 ] || [ -z "$err" ]; then err="$(status_code_msg "$status")"; fi
        logdebug "body: '$body'"
        add_problem "Given ASTRA_CLUSTER_ID did not pass validation: $err ($status)"
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
step_detect_resource_count() {
    local -r resource="$1"
    if ! str_matches_at_least_one "$resource" "namespaces" "nodes"; then
        fatal "invalid resource '$resource': only 'namespaces' or 'nodes' supported"
    fi

    _count="$(kubectl get "$resource" -o json | jq -r '.items | length' 2> /dev/null)"
    if [ -z "$_count" ] || [ "$_count" -lt 1 ]; then
        _count=""
    fi

    logdebug "Found '$_count' $resource."
    _return_value="$_count"
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

step_kustomize_global_pull_secret_if_needed() {
    local -r global_pull_secret="${1:-""}"
    local -r kustomization_file="${2}"
    local -r kustomization_dir="$(dirname "$kustomization_file")"
    local -r connector_namespace="$(get_connector_namespace)"
    local -r trident_namespace="$(get_trident_namespace)"
    local -r connector_registry="$(join_rpath "$CONNECTOR_IMAGE_REGISTRY" "$(get_base_repo "$CONNECTOR_IMAGE_REPO")")"
    local -r trident_acp_registry="$(join_rpath "$TRIDENT_ACP_IMAGE_REGISTRY" "$(get_base_repo "$TRIDENT_ACP_IMAGE_REPO")")"
    local -r encoded_creds=$(echo -n "$ASTRA_ACCOUNT_ID:$ASTRA_API_TOKEN" | base64)

    [ -z "$kustomization_file" ] && fatal "no kustomization file given"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"

       # SECRET GENERATOR
    cat <<EOF >> "$kustomization_file"
generatorOptions:
  disableNameSuffixHash: true
secretGenerator:
- name: astra-api-token
  namespace: "${connector_namespace}"
  literals:
  - apiToken="${ASTRA_API_TOKEN}"
EOF
    if [ -z "$global_pull_secret" ]; then
        # if image pull secret is empty, set same name for connector and trident secret so torc patch works as expected
        IMAGE_PULL_SECRET="astra-regcred"
        if components_include_connector; then
            cat <<EOF >> "$kustomization_file"
- name: "${IMAGE_PULL_SECRET}"
  namespace: "${connector_namespace}"
  type: kubernetes.io/dockerconfigjson
  literals:
  - |
    .dockerconfigjson={
      "auths": {
        "${connector_registry}": {
          "username": "$ASTRA_ACCOUNT_ID",
          "password": "$ASTRA_API_TOKEN",
          "auth": "${encoded_creds}"
        }
      }
    }
EOF
            logdebug "$kustomization_file: added connector secret to namespace $connector_namespace"
        fi
    
        if components_include_trident && [ "$trident_namespace" != "$connector_namespace" ]; then
            cat <<EOF >> "$kustomization_file"
- name: "${IMAGE_PULL_SECRET}"
  namespace: "${trident_namespace}"
  type: kubernetes.io/dockerconfigjson
  literals:
  - |
    .dockerconfigjson={
      "auths": {
        "${trident_acp_registry}": {
          "username": "$ASTRA_ACCOUNT_ID",
          "password": "$ASTRA_API_TOKEN",
          "auth": "${encoded_creds}"
        }
      }
    }
EOF
            logdebug "$kustomization_file: added trident acp secret to namespace $trident_namespace"
        fi
    fi

    

    insert_into_file_after_pattern "$kustomization_file" "patches:" '
- target:
    kind: Deployment
  patch: |-
    - op: replace
      path: /spec/template/spec/imagePullSecrets
      value:
        - name: "'"${global_pull_secret}"'"
'
    logdebug "$kustomization_file: added pull secret patch ($global_pull_secret)"
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
    local -r connector_autosupport_enrolled="$CONNECTOR_AUTOSUPPORT_ENROLLED"
    local -r connector_autosupport_url="$CONNECTOR_AUTOSUPPORT_URL"
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
        "- https://github.com/NetApp/astra-connector-operator/unified-installer/?ref=$__GIT_REF_CONNECTOR_OPERATOR"
    logdebug "$kustomization_file: added resources entry for connector kustomization"

    # Default memory limit
    memory_limit=2
    snapshot_count=""
    if prompt_user_yes_no "Do you anticipate having more than 10,000 snapshots and backups existing at the same time at any point? "; then
        prompt_user_number_greater_than_zero snapshot_count "Please estimate the maximum number of snapshots and backups you expect to have existing simultaneously within this cluster? (enter number value): "
        # Calculate estimated_memory and round up to the nearest integer
        estimated_memory=$(echo "$snapshot_count 5000" | awk '{printf("%d\n", ($1/$2)+0.6)}')

        # If estimated_memory is greater than memory_limit, set memory_limit to estimated_memory
        if (( estimated_memory > memory_limit )); then
            memory_limit=$estimated_memory
        fi
    fi
    loginfo "Memory limit set to: $memory_limit GB"

    # ASTRA CONNECTOR CR
    local labels_field_and_content_with_default=""
        if [ -n "$_PROCESSED_LABELS_WITH_DEFAULT" ]; then
            labels_field_and_content_with_default="${__NEWLINE}  labels:${__NEWLINE}${_PROCESSED_LABELS_WITH_DEFAULT}"
        fi
    local labels_field_and_content=""
    if [ -n "$_PROCESSED_LABELS" ]; then
        labels_field_and_content="${__NEWLINE}  labels:${__NEWLINE}${_PROCESSED_LABELS}"
    fi
    cat <<EOF > "$crs_file"
apiVersion: astra.netapp.io/v1
kind: AstraConnector
metadata:
  name: astra-connector
  namespace: "${connector_namespace}"${labels_field_and_content_with_default}
spec:
  astra:
    accountId: ${account_id}
    tokenRef: astra-api-token
    cloudId: ${cloud_id}
    clusterId: ${cluster_id}
    skipTLSValidation: ${skip_tls_validation}  # Should be set to false in production environments${labels_field_and_content}
  imageRegistry:
    name: "${connector_registry}"
    secret: "${connector_regcred_name}"
  autoSupport:
    enrolled: ${connector_autosupport_enrolled}
    url: ${connector_autosupport_url}
  natsSyncClient:
    cloudBridgeURL: ${astra_url}
  neptune:
    resources:
      limits:
        memory: ${memory_limit}Gi
      requests:
        cpu: ".5"
        memory: ${memory_limit}Gi
EOF
    if [ -n "$host_alias_ip" ]; then
        echo "    hostAliasIP: $host_alias_ip" >> "$crs_file"
    fi

    if [ -n "$connector_tag" ]; then
      echo "  astraConnect:" >> "$crs_file"
      echo "    image: \"${connector_tag}\" # This field sets the tag, not the image" >> "$crs_file"
    fi

    if [ -n "$neptune_tag" ]; then
      echo "  neptune:" >> "$crs_file"
      echo "    image: \"${neptune_tag}\" # This field sets the tag, not the image" >> "$crs_file"
    fi

    echo "---" >> "$crs_file"

    logdebug "$crs_file: OK"
    logdebug "$crs_file: added AstraConnector CR"
}

step_collect_existing_trident_info() {
    logheader $__INFO "Checking if Trident is installed..."
    _TRIDENT_COLLECTION_STEP_CALLED="true"

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
        loginfo "* Trident not found -- it will be installed."
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
    fi
    _EXISTING_TORC_NAME="$(echo "$torc_json" | jq -r '.metadata.name')"
    _EXISTING_TRIDENT_NAMESPACE="$trident_ns"
    loginfo "* Trident namespace: '$_EXISTING_TRIDENT_NAMESPACE'"

    # Trident image
    local -r trident_image="$(echo "$torc_json" | jq -r ".spec.tridentImage" 2> /dev/null)"
    if [ -n "$trident_image" ] && [ "$trident_image" != "null" ]; then
        _EXISTING_TRIDENT_IMAGE="$trident_image"
        logdebug "trident image: $trident_image"
    else
        logdebug "trident image: not found"
    fi

    # Trident version
    local -r trident_version="$(kubectl get tridentversions -n trident -o json | jq -r '.items.[0].trident_version' 2> /dev/null)"
    if [ -z "$trident_version" ]; then
        logwarn "Failed to resolve existing Trident version. ACP may not be supported without an upgrade!"
    else
        _EXISTING_TRIDENT_VERSION="$trident_version"
        loginfo "* Trident version: $trident_version"
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
    if [ -n "$acp_image" ] && [ "$acp_image" != "null" ]; then
        logdebug "trident ACP image: $acp_image"
        _EXISTING_TRIDENT_ACP_IMAGE="$acp_image"
    else
        logdebug "trident ACP image: not found"
    fi

    # Trident operator
    local -r trident_operator_json="$(kubectl get deploy/trident-operator -n "$trident_ns" -o json 2> /dev/null)"
    if [ -n "$trident_operator_json" ]; then
        local -r containers_length="$(echo "$trident_operator_json" | jq -r '.spec.template.spec.containers | length' 2> /dev/null)"
        # Assume there's only one container (and it's the trident-operator). Not great, but if that ever changes,
        # we'll at least learn about it.
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

step_existing_trident_flags_compatibility_check() {
    [ "$COMPONENTS" == "$__COMPONENTS_ALL_ASTRA_CONTROL" ] && return 0
    ! existing_trident_needs_modifications && return 0

    local msg="Existing Trident install requires an upgrade but DO_NOT_MODIFY_EXISTING_TRIDENT=true,"
    msg+=" and no other valid operations can be done due to COMPONENTS=$COMPONENTS."
    add_problem "$msg"
    return 1
}

step_generate_trident_fresh_install_yaml() {
    local -r kustomization_file="$__GENERATED_KUSTOMIZATION_FILE"
    local -r crs_file="$__GENERATED_CRS_FILE"
    [ ! -f "$kustomization_file" ] && fatal "kustomization file '$kustomization_file' does not exist"
    [ ! -f "$crs_file" ] && touch "$crs_file"

    logheader $__DEBUG "Generating Trident YAML files..."

    # TODO point to https://github.com/NetApp/trident when 24.06 image is available
    insert_into_file_after_pattern "$kustomization_file" "resources:" \
        "- https://github.com/NetApp/astra-connector-operator/trident-temp/deploy?ref=$__GIT_REF_TRIDENT"
    logdebug "$kustomization_file: added resources entry for trident operator"

    local -r torc_name="$__DEFAULT_TORC_NAME"
    local -r trident_image="$(get_config_trident_image)"
    local -r autosupport_image="$(get_config_trident_autosupport_image)"
    local -r acp_image="$(get_config_acp_image)"
    local -r namespace="$(get_trident_namespace)"
    local pull_secret="[]"
    local enable_acp="true"
    local labels_field_and_content=""
    if [ -n "$IMAGE_PULL_SECRET" ]; then pull_secret='["'$IMAGE_PULL_SECRET'"]'; fi
    if [ -n "$_PROCESSED_LABELS_WITH_DEFAULT" ]; then
        labels_field_and_content="${__NEWLINE}  labels:${__NEWLINE}${_PROCESSED_LABELS_WITH_DEFAULT}"
    fi

    cat <<EOF >> "$crs_file"
apiVersion: trident.netapp.io/v1
kind: TridentOrchestrator
metadata:
  name: "$torc_name"
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
    local -r image_patch='{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"'"$new_image"'"}'
    local -a patch_list="$image_patch"

    # Update image pull secrets if needed
    if [ -n "$IMAGE_PULL_SECRET" ]; then
        if echo "$_EXISTING_TRIDENT_OPERATOR_PULL_SECRETS" | grep -q "^${IMAGE_PULL_SECRET}$" &> /dev/null; then
            logdebug "image pull secret '$IMAGE_PULL_SECRET' already present in trident-operator"
        else
            if [ -z "$_EXISTING_TRIDENT_OPERATOR_PULL_SECRETS" ]; then
                patch_list+=',{"op":"replace","path":"/spec/template/spec/imagePullSecrets","value":[{"name":"'"$IMAGE_PULL_SECRET"'"}]}'
            else
                patch_list+=',{"op":"add","path":"/spec/template/spec/imagePullSecrets/-","value":{"name":"'"$IMAGE_PULL_SECRET"'"}}'
            fi
        fi
    fi

    if [ -n "$patch_list" ]; then
        echo "[${patch_list%,}]"
        patch_list="'$(echo "[${patch_list%,}]" | jq '.')'"
        _PATCHES_TRIDENT_OPERATOR+=("deploy/trident-operator -n '$namespace' --type=json -p $patch_list")
    fi
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

    # Update image pull secrets if needed
    if [ -n "$IMAGE_PULL_SECRET" ]; then
        if echo "$_EXISTING_TORC_PULL_SECRETS" | grep -q "^${IMAGE_PULL_SECRET}$" &> /dev/null; then
            logdebug "image pull secret '$IMAGE_PULL_SECRET' already present in torc"
        else
            if [ -z "$_EXISTING_TRIDENT_OPERATOR_PULL_SECRETS" ]; then
                torc_patch_list+='{"op":"replace","path":"/spec/imagePullSecrets","value":["'"$IMAGE_PULL_SECRET"'"]},'
            else
                torc_patch_list+='{"op":"add","path":"/spec/imagePullSecrets/-","value":"'"$IMAGE_PULL_SECRET"'"},'
            fi
        fi
    fi

    if [ -n "$torc_patch_list" ]; then
        torc_patch_list="'$(echo "[${torc_patch_list%,}]" | jq '.')'"
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
    insert_into_file_after_pattern "${kustomization_file}" "kind: Kustomization" "${content}"

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
    local captured_err=""
    if ! is_dry_run; then
        output="$(kubectl apply -k "$operators_dir" 2> "$__ERR_FILE")"
        captured_err="$(get_captured_err)"
        if echo "$captured_err" | grep -q "Warning:"; then
            logdebug "captured warning when applying kustomize resources:${__NEWLINE}$captured_err"
        elif echo "$captured_err" | grep -q "no objects passed to apply"; then
            logdebug "no kustomize resources to apply, skipping"
        elif [ -z "$output" ] || [ -n "$captured_err" ]; then
            add_problem "Failed to apply kustomize resources: $captured_err"
        fi
        logdebug "kustomize apply output:${__NEWLINE}$output"
    fi
    exit_if_problems
    loginfo "* Astra operators have been applied to the cluster."

    # Apply CRs (if we have any)
    if [ -f "$crs_file_path" ]; then
        logdebug "apply CRs"
        if ! is_dry_run; then
            output="$(kubectl apply -f "$crs_file_path" 2> "$__ERR_FILE")"
            captured_err="$(get_captured_err)"
            if [ -z "$output" ] || [ -n "$captured_err" ]; then
                add_problem "Failed to apply CRs: $captured_err"
            fi
            logdebug "$output"
        else
            logdebug "skipped due to dry run"
        fi
        loginfo "* Astra CRs have been applied to the cluster."
    else
        logdebug "No CRs file to apply"
    fi
    exit_if_problems
}

step_apply_trident_operator_patches() {
    logheader "$__DEBUG" "$(prefix_dryrun "Applying Trident Operator patches...")"
    local -ra patches=("${_PATCHES_TRIDENT_OPERATOR[@]}")
    local -r patches_len="${#patches[@]}"

    if ! trident_will_be_installed_or_modified && [ "$patches_len" -gt 0 ]; then
        fatal "found $patches_len operator patches (expected 0) despite trident not being installed or modified"
    fi

    if debug_is_on && [ "$patches_len" -gt 0 ]; then
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
    local -r patches_len="${#patches[@]}"

    if ! trident_will_be_installed_or_modified && [ "$patches_len" -gt 0 ]; then
        fatal "found $patches_len torc patches (expected 0) despite trident not being installed or modified"
    fi

    if debug_is_on && [ "$patches_len" -gt 0 ]; then
        local disclaimer="# THIS FILE IS FOR DEBUGGING PURPOSES ONLY. These are the patches applied to the"
        disclaimer+="${__NEWLINE}# TridentOrchestrator resource when upgrading Trident or enabling ACP."
        disclaimer+="${__NEWLINE}"
        append_lines_to_file "${__GENERATED_PATCHES_TORC_FILE}" "$disclaimer" "${patches[@]}"
    fi

    # Take a backup of the TORC just in case
    if [ -n "$_EXISTING_TORC_NAME" ]; then
        local -r backup="$(backup_kubernetes_resource "tridentorchestrator" "$_EXISTING_TORC_NAME" "$__GENERATED_CRS_DIR")"
        if [ -n "$backup" ]; then
            loginfo "* Created backup for TridentOrchestrator '$_EXISTING_TORC_NAME': '$backup'"
        elif [ -n "$_return_error" ]; then
            logdebug "failed to create backup for TridentOrchestrator '$_EXISTING_TORC_NAME': $_return_error"
        else
            logdebug "failed to create backup for TridentOrchestrator '$_EXISTING_TORC_NAME': unknown error"
        fi
    fi

    apply_kubectl_patches "${patches[@]}"
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
        elif ! wait_for_deployment_running "operator-controller-manager" "$connector_operator_ns" "3"; then
            add_problem "connector operator deploy: failed" "The Astra Connector Operator failed to deploy"
        elif ! wait_for_deployment_running "neptune-controller-manager" "$connector_ns" "3"; then
            add_problem "neptune deploy: failed" "Neptune failed to deploy"
        elif ! wait_for_deployment_running "astraconnect" "$connector_ns" "5"; then
            add_problem "astraconnect deploy: failed" "The Astra Connector failed to deploy"
        elif ! wait_for_cr_state "astraconnectors/astra-connector" ".status.natsSyncClient.status" "Registered with Astra" "$connector_ns"; then
            add_problem "cluster registration: failed" "Cluster registration failed"
        fi
    fi

    local -r torc_name="${_EXISTING_TORC_NAME:-"$__DEFAULT_TORC_NAME"}"
    if trident_will_be_installed_or_modified; then
        if is_dry_run; then
            logdebug "skip monitoring trident components because it's a dry run"
        elif ! wait_for_deployment_running "trident-operator" "$trident_ns" "3"; then
            add_problem "trident operator: failed" "The Trident Operator failed to deploy"
        elif ! wait_for_cr_state "torc/$torc_name" ".status.status" "Installed" "$trident_ns" "12"; then
            add_problem "trident: failed" "Trident failed to deploy: status never reached 'Installed'"
        fi
    fi
}

step_cleanup_tmp_files() {
    debug_is_on && logdebug "last captured err: '$(get_captured_err)'"
    rm -f "$__ERR_FILE" &> /dev/null
}

#======================================================================
#== Main
#======================================================================
set_log_level
logln $__INFO "====== Astra Cluster Installer ${__RELEASE_VERSION} ======"
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
step_check_volumesnapshotclasses

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
    exit_if_problems
    step_check_astra_cloud_and_cluster_id
else
    logdebug "skipping all Astra checks (COMPONENTS=${COMPONENTS}, SKIP_ASTRA_CHECK=${SKIP_ASTRA_CHECK})"
fi
exit_if_problems

# ------------ YAML GENERATION ------------
step_check_kubeconfig_choice
step_init_generated_dirs_and_files
step_kustomize_global_namespace_if_needed "$NAMESPACE" "$__GENERATED_KUSTOMIZATION_FILE"
step_kustomize_global_pull_secret_if_needed "$IMAGE_PULL_SECRET" "$__GENERATED_KUSTOMIZATION_FILE"

# CONNECTOR yaml
if components_include_connector; then
    step_generate_astra_connector_yaml "$__GENERATED_CRS_DIR" "$__GENERATED_OPERATORS_DIR"
fi

# TRIDENT / ACP yaml
step_collect_existing_trident_info
exit_if_problems

step_existing_trident_flags_compatibility_check
exit_if_problems

if trident_will_be_installed_or_modified; then
    if trident_is_missing; then
        step_generate_trident_fresh_install_yaml
    elif [ -z "$_EXISTING_TRIDENT_OPERATOR_IMAGE" ]; then
        logwarn "Upgrading Trident without the Trident Operator is not currently supported, skipping."
    elif existing_trident_can_be_modified; then
        # Upgrade Trident/Operator?
        if components_include_trident; then
            # Trident upgrade (includes operator upgrade if needed)
            if trident_image_needs_upgraded; then
                # if trident version < 23.10
                if version_higher_or_equal "23.07" "$_EXISTING_TRIDENT_VERSION"; then
                    logwarn "Your Trident installation is at version $_EXISTING_TRIDENT_VERSION, while the lowest required version to enable ACP is 23.10."
                fi

                if config_trident_image_is_custom; then
                    local -r custom_image="$(as_full_image "$TRIDENT_IMAGE_REGISTRY" "$TRIDENT_IMAGE_REPO" "$TRIDENT_IMAGE_TAG")"
                    local warning_message="Warning: we cannot verify the version of the custom Trident image you provided"
                    warning_message += " ($custom_image). ACP support cannot be guaranteed if upgrading to that image."
                    logwarn "$warning_message"
                fi

                if ! components_include_connector && (components_include_trident || components_include_acp); then
                    if prompt_user_yes_no "Would you like to upgrade Trident? Choosing no will exit the script (yes/no):"; then
                        step_generate_torc_patch "$_EXISTING_TORC_NAME" "$(get_config_trident_image)" "" "" "$(get_config_trident_autosupport_image)"
                        trident_operator_image_needs_upgraded && step_generate_trident_operator_patch
                    else
                        exit 0
                    fi
                fi

                if components_include_connector; then
                    if acp_is_enabled; then
                        if prompt_user_yes_no "Would you like to upgrade Trident?"; then
                            step_generate_torc_patch "$_EXISTING_TORC_NAME" "$(get_config_trident_image)" "" "" "$(get_config_trident_autosupport_image)"
                            trident_operator_image_needs_upgraded && step_generate_trident_operator_patch
                        else
                            loginfo "Trident will not be upgraded"
                        fi
                    else
                        if prompt_user_yes_no "Would you like to upgrade Trident? If you choose no, ACP will remain disabled"; then
                            step_generate_torc_patch "$_EXISTING_TORC_NAME" "$(get_config_trident_image)" "" "" "$(get_config_trident_autosupport_image)"
                            trident_operator_image_needs_upgraded && step_generate_trident_operator_patch
                        else
                            loginfo "Trident will not be upgraded and ACP will remain disabled."
                        fi
                    fi
                fi
            # Trident operator upgrade (standalone)
            elif trident_operator_image_needs_upgraded; then
                if config_trident_operator_image_is_custom || prompt_user_yes_no "Would you like to upgrade the Trident Operator?"; then
                    step_generate_trident_operator_patch
                else
                    loginfo "Trident Operator will not be upgraded."
                fi
            fi
        else
            logdebug "Skipping Trident upgrade (COMPONENTS=${COMPONENTS}, DO_NOT_MODIFY_EXISTING_TRIDENT=${DO_NOT_MODIFY_EXISTING_TRIDENT})"
        fi

        # Upgrade/Enable ACP?
        if components_include_acp; then
            # Enable ACP if needed (includes ACP upgrade)
            if ! acp_is_enabled; then
                if config_acp_image_is_custom || prompt_user_yes_no "Would you like to enable ACP?"; then
                    # create trident-acp secret
                    kubectl create secret docker-registry "$IMAGE_PULL_SECRET" --docker-username="$ASTRA_ACCOUNT_ID" --docker-password="$ASTRA_API_TOKEN" -n trident --docker-server="$TRIDENT_ACP_IMAGE_REGISTRY"
                    step_generate_torc_patch "$_EXISTING_TORC_NAME" "" "$(get_config_acp_image)" "true"
                else
                    loginfo "ACP will not be enabled."
                fi
            # ACP upgrade (ACP already enabled)
            elif acp_image_needs_upgraded; then
                if config_acp_image_is_custom || prompt_user_yes_no "Would you like to upgrade ACP?"; then
                    # create trident-acp secret
                    kubectl create secret docker-registry "$IMAGE_PULL_SECRET" --docker-username="$ASTRA_ACCOUNT_ID" --docker-password="$ASTRA_API_TOKEN" -n trident --docker-server="$TRIDENT_ACP_IMAGE_REGISTRY"
                    step_generate_torc_patch "$_EXISTING_TORC_NAME" "" "$(get_config_acp_image)" "true"
                else
                    loginfo "ACP will not be upgraded."
                fi
            fi
        else
            logdebug "Skipping ACP changes (COMPONENTS=${COMPONENTS})"
        fi
    fi
fi

# IMAGE REMAPS, LABELS, RESOURCE LIMITS yaml
step_add_labels_to_kustomization "${_PROCESSED_LABELS_WITH_DEFAULT}" "${__GENERATED_KUSTOMIZATION_FILE}" "${__GENERATED_CRS_FILE}"
step_add_image_remaps_to_kustomization
exit_if_problems

# ------------ DEPLOYMENT ------------
step_apply_resources
step_apply_trident_operator_patches
step_apply_torc_patches
step_monitor_deployment_progress
exit_if_problems

if ! is_dry_run; then
    logheader $__INFO "Cluster management complete!"
else
    debug_is_on && print_built_config
    logheader $__INFO "$(prefix_dryrun "See generated files")"
    loginfo "$(find "$__GENERATED_CRS_DIR" -type f)"
    _msg="You can run 'kustomize build $__GENERATED_OPERATORS_DIR > $__GENERATED_OPERATORS_DIR/resources.yaml'"
    _msg+=" to view the CRDs and operator resources that will be applied."
    logln $__INFO "$_msg"
    exit 0
fi

step_cleanup_tmp_files

