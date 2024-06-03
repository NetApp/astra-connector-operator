#!/bin/bash

fatal() {
    echo "$1"
    exit 1
}

k8s_get_resource() {
    local -r resource="$1"
    local -r namespace="${2:-""}"
    local -r format="${3:-"json"}"

    [ -z "$resource" ] && fatal "no resource given"

    local -a args=()
    [ -n "$namespace" ] && args+=("-n" "$namespace")

    local -r output="$(kubectl get "$resource" "${args[@]}" -o "$format")"
    if [ -n "$output" ] && [ -z "$captured_err" ]; then
        echo "$output"
        return 0
    fi

    return 1
}

k8s_resource_exists() {
    local -r resource="$1"
    local -r namespace="$2"

    if k8s_get_resource "$resource" "$namespace" 1> /dev/null; then
        return 0
    fi

    return 1
}

wait_for_resource_created() {
    local -r resource="$1"
    local -r namespace="$2"
    local -r timeout="${3:-120}"
    local -r wait_for_delete_instead="${4:-"false"}"

    [ -z "$resource" ] && fatal "no resource given"
    [ -z "$namespace" ] && fatal "no namespace given"

    local max_checks=1
    if (( timeout > 0 )); then
        max_checks=$(( timeout / 5 ))
        if (( max_checks <= 0 )); then
            max_checks=1
        fi
    fi

    echo "waiting for resource '$resource' in namespace '$namespace' to be created/deleted (timeout=$timeout)"
    local counter=0
    while ((counter < max_checks)); do
        if k8s_resource_exists "$resource" "$namespace"; then
            echo "resource '$resource' found"
            if [ "$wait_for_delete_instead" != "true" ]; then
                return 0
            else
                echo "resource '$resource' not yet deleted"
                ((counter++))
                sleep 5
            fi
        else
            if [ "$wait_for_delete_instead" == "true" ]; then
                return 0
            else
                echo "resource '$resource' not yet created"
                ((counter++))
                sleep 5
            fi
        fi
    done

    return 1
}

wait_for_resource_deleted() {
    wait_for_resource_created "$1" "$2" "$3" "true"
}

cleanup_trident() {
    echo && echo && echo "~~~~~~~~~ Clean-up Trident"
    local -r namespace="${1:-"trident"}"

    kubectl delete torc trident
    wait_for_resource_deleted "deploy/trident-controller" "$namespace"
    kubectl delete deploy/trident-operator -n "$namespace"
    wait_for_resource_deleted "deploy/trident-operator" "$namespace"
}

cleanup_connector() {
    echo && echo && echo "~~~~~~~~~ Clean-up Connector"
    local -r namespace="${1:-astra}"
    kubectl -n "$namespace" delete astraconnectors/astra-connector
    kubectl -n "$namespace" delete deploy/operator-controller-manager
    kubectl -n "$namespace" delete service/operator-controller-manager-metrics-service
    kubectl get crd | grep astra  | awk '{print $1}' | grep -v astraconnectors.astra.netapp.io \
        | xargs -I{} kubectl get {} -n "$namespace" -o name  | xargs -I{} kubectl delete {} -n "$namespace"

    wait_for_resource_deleted "astraconnectors/astra-connector" "$namespace"
    wait_for_resource_deleted "deploy/operator-controller-manager" "$namespace"
    wait_for_resource_deleted "service/operator-controller-manager-metrics-service" "$namespace"
}

store_trident_yaml() {
    _torc_yaml_1="$(k8s_get_resource "torc/trident" "" "yaml")"
    _top_yaml_1="$(k8s_get_resource "deploy/trident-operator" "$TRIDENT_NS" "yaml")"
}

check_if_trident_yaml_changed() {
    _torc_yaml_2="$(k8s_get_resource "torc/trident" "" "yaml")"
    _top_yaml_2="$(k8s_get_resource "deploy/trident-operator" "$TRIDENT_NS" "yaml")"
    _failed="false"
    [ "$_torc_yaml_1" != "$_torc_yaml_2" ] && echo "fail: the torc changed" && _failed="true"
    [ "$_top_yaml_1" != "$_top_yaml_2" ] && echo "fail: the trident-operator changed" && _failed="true"
    [ "$_failed" == "true" ] && exit 1
}

run_install_script() {
    if ! CONFIG_FILE="$CONFIG_FILE" "$__SCRIPT_DIR"/install.sh; then
        echo "failed: script exited with non-zero code"
        exit 1
    else
        echo "script completed successfully"
        return 0
    fi
}

should_run_test() {
    [ -z "$__tests_to_run" ] && return 0
    echo "$__tests_to_run" | grep -qw "$1"
}

#==========================================================================================
#-- CONFIG
#==========================================================================================
__tests_to_run="${*}"
__SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "Config file: $CONFIG_FILE"
source $CONFIG_FILE
GLOBAL_NS="${NAMESPACE:-"astra"}"
TRIDENT_NS="${TRIDENT_NAMESPACE:-${GLOBAL_NS:-"trident"}}"
CONNECTOR_NS="${CONNECTOR_NAMESPACE:-${GLOBAL_NS:-"astra-connector"}}"
_old_disable_prompts="$DISABLE_PROMPTS"
_old_limits_preset="$RESOURCE_LIMITS_PRESET"
echo "Global namespace: $GLOBAL_NS"
echo "Connector namespace: $CONNECTOR_NS"
echo "Trident namespace: $TRIDENT_NS"

#==========================================================================================
#-- TESTS
#==========================================================================================
_test_number=1
if should_run_test "${_test_number}"; then
    echo && echo && echo "================== Starting Test ${_test_number}: Greenfield"
    _setup_log="log-test${_test_number}-setup.txt"
    cleanup_trident "$GLOBAL_NS" 2> "$_setup_log"
    cleanup_trident "$TRIDENT_NS" 2> "$_setup_log"
    cleanup_connector "$CONNECTOR_NS" 2> "$_setup_log"

    export DISABLE_PROMPTS=true
    echo && echo && echo "~~~~~~~~~ Run test (dry run)"
    export COMPONENTS=ALL_ASTRA_CONTROL
    export DRY_RUN=true
    run_install_script
    export DISABLE_PROMPTS="$_old_disable_prompts"

    echo && echo && echo "~~~~~~~~~ Run test"
    export DRY_RUN=false
    run_install_script
    echo "test ${_test_number} successful"
fi

# For this test, if you want to check whether Trident/Trident-operator do not get modified when the
# user chooses not to upgrade, make sure you select "no" as well. You should also make sure the resource
# limit preset during setup is different than the one during the test itself, to make sure resource
# limits also don't get applied when the upgrade is declined.
_test_number=2
if should_run_test ${_test_number}; then
    echo && echo && echo "================== Starting Test ${_test_number}: Brownfield"
    _setup_log="log-test${_test_number}-setup.txt"
    cleanup_trident "$GLOBAL_NS" 2> "$_setup_log"
    cleanup_trident "$TRIDENT_NS" 2> "$_setup_log"
    cleanup_connector "$CONNECTOR_NS" 2> "$_setup_log"

    # Setup
    export DISABLE_PROMPTS=true
    export RESOURCE_LIMITS_PRESET=large
    echo && echo && echo "~~~~~~~~~ Setup Brownfield Trident (dry run)"
    export __TRIDENT_VERSION_OVERRIDE=23.07
    export COMPONENTS=TRIDENT_ONLY
    export NAMESPACE="$TRIDENT_NS"
    export DRY_RUN=true
    run_install_script

    echo && echo && echo "~~~~~~~~~ Setup Brownfield Trident"
    export DRY_RUN=false
    run_install_script
    export RESOURCE_LIMITS_PRESET="$_old_limits_preset"

    # Test
    echo && echo && echo "~~~~~~~~~ Run test (dry run)"
    export __TRIDENT_VERSION_OVERRIDE=24.02
    export COMPONENTS=ALL_ASTRA_CONTROL
    export NAMESPACE="$GLOBAL_NS"
    export DRY_RUN=true
    run_install_script
    export DISABLE_PROMPTS="$_old_disable_prompts"

    echo && echo && echo "~~~~~~~~~ Run test"
    _test_log="log-test${_test_number}-test.txt"
    export DRY_RUN=false
    store_trident_yaml
    run_install_script | tee -a "$_test_log"

    if grep -q "WARNING: You have chosen to use a version of Trident" < "$_test_log"; then
        check_if_trident_yaml_changed
    fi
    echo "test ${_test_number} successful"
fi

_test_number=3
if should_run_test $_test_number; then
    echo && echo && echo "================== Starting Test $_test_number: Brownfield, no modify Trident"
    _setup_log="log-test${_test_number}-setup.txt"
    cleanup_trident "$GLOBAL_NS" 2> "$_setup_log"
    cleanup_trident "$TRIDENT_NS" 2> "$_setup_log"
    cleanup_connector "$CONNECTOR_NS" 2> "$_setup_log"

    # Setup
    export DISABLE_PROMPTS=true
    echo && echo && echo "~~~~~~~~~ Setup Brownfield Trident (dry run)"
    export __TRIDENT_VERSION_OVERRIDE=23.07
    export COMPONENTS=TRIDENT_ONLY
    export NAMESPACE="$TRIDENT_NS"
    export DRY_RUN=true
    run_install_script

    echo && echo && echo "~~~~~~~~~ Setup Brownfield Trident"
    export DRY_RUN=false
    run_install_script

    # Test (dry run)
    echo && echo && echo "~~~~~~~~~ Run test (dry run)"
    export __TRIDENT_VERSION_OVERRIDE=24.02
    export COMPONENTS=ALL_ASTRA_CONTROL
    export NAMESPACE="$GLOBAL_NS"
    export DO_NOT_MODIFY_EXISTING_TRIDENT=true
    export DRY_RUN=true
    run_install_script
    export DISABLE_PROMPTS="$_old_disable_prompts"

    echo && echo && echo "~~~~~~~~~ Run test"
    export DRY_RUN=false
    store_trident_yaml
    run_install_script
    check_if_trident_yaml_changed
    echo "test ${_test_number} successful"
fi
