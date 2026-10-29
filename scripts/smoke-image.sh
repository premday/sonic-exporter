#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_IMAGE_NAME="sonic-exporter"
DEFAULT_IMAGE_TAG="smoke-test"
DEFAULT_PORT="19101"
IMAGE_NAME="${IMAGE_NAME-${DEFAULT_IMAGE_NAME}}"
IMAGE_TAG="${IMAGE_TAG-${DEFAULT_IMAGE_TAG}}"
IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"
PORT="${DEFAULT_PORT}"
DRY_RUN="false"
BUILD_IMAGE="true"
CONTAINER_NAME="sonic-exporter-smoke-$$"
CONTAINER_STARTED="false"
METRICS_FILE=""
SUPERVISOR_STATUS=""

usage() {
    cat <<'EOF'
Usage: scripts/smoke-image.sh [--image <image>] [--no-build] [--port <host_port>] [--dry-run]

Builds or reuses a local Docker image, checks Supervisor, and checks /metrics.
EOF
}

print_cmd() {
    local -a cmd=()
    local arg

    for arg in "$@"; do
        cmd+=("$(printf '%q' "$arg")")
    done

    printf '%s\n' "${cmd[*]}"
}

die() {
    printf 'Error: %s\n' "$1" >&2
    exit 1
}

cleanup() {
    local status=$?

    if [[ -n "${METRICS_FILE}" && -f "${METRICS_FILE}" ]]; then
        if ! rm -f "${METRICS_FILE}"; then
            printf 'Failed to remove temporary metrics file: %s\n' "${METRICS_FILE}" >&2
            status=1
        fi
    fi

    if [[ "${CONTAINER_STARTED}" == "true" ]]; then
        if ! docker rm -f "${CONTAINER_NAME}" >/dev/null; then
            printf 'Failed to remove smoke container: %s\n' "${CONTAINER_NAME}" >&2
            status=1
        fi
    fi

    exit "${status}"
}

print_failure_logs() {
    printf 'Container logs from %s:\n' "${CONTAINER_NAME}" >&2
    if ! docker logs --tail 100 "${CONTAINER_NAME}" >&2; then
        printf 'Could not read smoke container logs.\n' >&2
    fi
}

trap cleanup EXIT

while (($# > 0)); do
    case "$1" in
        --image)
            [[ $# -ge 2 ]] || die "--image requires a value"
            IMAGE_REF="$2"
            BUILD_IMAGE="false"
            shift 2
            ;;
        --no-build)
            BUILD_IMAGE="false"
            shift
            ;;
        --port)
            [[ $# -ge 2 ]] || die "--port requires a value"
            PORT="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            die "unknown argument: $1"
            ;;
    esac
done

[[ "${PORT}" =~ ^[0-9]+$ ]] || die "--port must be a number"
[[ "${PORT}" != "0" ]] || die "--port must be greater than zero"
[[ -n "${IMAGE_REF}" ]] || die "image must not be empty"

BUILD_SCRIPT="${REPO_ROOT}/scripts/build-image.sh"
RUN_CMD=(docker run -d --name "${CONTAINER_NAME}" --cap-drop ALL --cap-add NET_RAW -p "127.0.0.1:${PORT}:9101" -e SONIC_EXPORTER_WEB_VRF= -e SONIC_EXPORTER_ROOTFS=/ "${IMAGE_REF}")
STATUS_CMD=(docker exec "${CONTAINER_NAME}" bash -c "supervisorctl status")
CURL_CMD=(curl -fsS --connect-timeout 2 --max-time 5 "http://127.0.0.1:${PORT}/metrics")
LOGS_CMD=(docker logs --tail 100 "${CONTAINER_NAME}")
RM_CMD=(docker rm -f "${CONTAINER_NAME}")

if [[ "${DRY_RUN}" == "true" ]]; then
    if [[ "${BUILD_IMAGE}" == "true" ]]; then
        print_cmd IMAGE_NAME="${IMAGE_NAME}" IMAGE_TAG="${IMAGE_TAG}" bash "${BUILD_SCRIPT}" --dry-run
    else
        printf '%s\n' "# using existing image: ${IMAGE_REF}"
    fi
    print_cmd "${RUN_CMD[@]}"
    print_cmd "${STATUS_CMD[@]}"
    print_cmd "${CURL_CMD[@]}"
    print_cmd "${LOGS_CMD[@]}"
    print_cmd "${RM_CMD[@]}"
    exit 0
fi

if [[ "${BUILD_IMAGE}" == "true" ]]; then
    IMAGE_NAME="${IMAGE_NAME}" IMAGE_TAG="${IMAGE_TAG}" bash "${BUILD_SCRIPT}"
fi

METRICS_FILE="$(mktemp)"

if ! "${RUN_CMD[@]}" >/dev/null; then
    print_failure_logs
    printf 'Launch failed; inspect %s for any stopped container left by Docker.\n' "${CONTAINER_NAME}" >&2
    exit 1
fi
CONTAINER_STARTED="true"

attempt=1
max_attempts=30
success="false"

while (( attempt <= max_attempts )); do
    if "${CURL_CMD[@]}" >"${METRICS_FILE}"; then
        if grep -q '^# HELP ' "${METRICS_FILE}"; then
            if SUPERVISOR_STATUS="$("${STATUS_CMD[@]}")"; then
                if [[ "${SUPERVISOR_STATUS}" =~ ^sonic-exporter[[:space:]]+RUNNING[[:space:]] ]]; then
                    success="true"
                    break
                fi
            fi
            printf 'Waiting for Supervisor: %s\n' "${SUPERVISOR_STATUS}" >&2
        fi
    fi

    sleep 1
    attempt=$((attempt + 1))
done

if [[ "${success}" != "true" ]]; then
    printf 'Smoke test failed after %s attempts\n' "${max_attempts}" >&2
    print_failure_logs
    exit 1
fi

if [[ "${BUILD_IMAGE}" == "true" ]] && ! grep '^sonic_exporter_build_info{' "${METRICS_FILE}" | grep -Fq "version=\"${IMAGE_TAG}\""; then
    printf 'Smoke test build info did not contain version="%s"\n' "${IMAGE_TAG}" >&2
    print_failure_logs
    exit 1
fi

printf 'Smoke test passed: Supervisor RUNNING and http://127.0.0.1:%s/metrics available\n' "${PORT}"
