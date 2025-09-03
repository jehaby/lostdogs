#!/usr/bin/env bash
set -euo pipefail

# Deploy using docker compose with a provided env file and local compose.yml.
# Simple and predictable: copy image tar, env, and compose.yml to remote, then compose up.

require_bin() { command -v "$1" >/dev/null 2>&1 || { echo "Missing required command: $1" >&2; exit 1; }; }

usage() {
  cat <<USAGE
Deploy lostdogs via docker compose on a remote host.

Required:
  --env-file PATH   Path to env file providing REMOTE_USER, REMOTE_HOST, VK_TOKEN, TG_TOKEN, TG_CHAT, etc.

Optional:
  --image IMAGE     Override IMAGE for this deploy
  --remote-path DIR Remote directory on the host (default: /home/$REMOTE_USER/infra/lostdogs)
  --platform VAL    Build platform (default: linux/amd64), requires buildx
  --dockerfile F    Dockerfile path (default: ./Dockerfile)
  --set KEY=VAL     Override/add any env variable (repeatable)

Examples:
  script/deploy.sh --env-file ./deploy.env
  script/deploy.sh --env-file ./deploy.env --image jehaby/lostdogs:2025-09-01 --set LOG_LEVEL=debug --set TG_ENABLED=true
  script/deploy.sh --env-file ./deploy.env --platform linux/amd64
USAGE
}

require_bin docker
require_bin scp
require_bin ssh

ENV_FILE=""
LOCAL_IMAGE=""
REMOTE_PATH=""
PLATFORM="linux/amd64"
DOCKERFILE="Dockerfile"
declare -a SET_OVERRIDES

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) ENV_FILE="$2"; shift 2;;
    --image) LOCAL_IMAGE="$2"; shift 2;;
    --remote-path) REMOTE_PATH="$2"; shift 2;;
    --platform) PLATFORM="$2"; shift 2;;
    --dockerfile) DOCKERFILE="$2"; shift 2;;
    --set) SET_OVERRIDES+=("$2"); shift 2;;
    -h|--help) usage; exit 0;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2;;
  esac
done

[[ -n "${ENV_FILE}" && -f "${ENV_FILE}" ]] || { echo "--env-file is required and must exist" >&2; usage; exit 2; }

# Merge env: base file + optional overrides and image
TMP_ENV_FILE="$(mktemp -t lostdogs.deploy.env.XXXXXXXX)"
cp "${ENV_FILE}" "${TMP_ENV_FILE}"
[[ -n "${LOCAL_IMAGE}" ]] && echo "IMAGE=${LOCAL_IMAGE}" >> "${TMP_ENV_FILE}"
for kv in "${SET_OVERRIDES[@]:-}"; do echo "$kv" >> "${TMP_ENV_FILE}"; done

# Load merged env locally for checks
set -a; . "${TMP_ENV_FILE}"; set +a

REMOTE_PATH="${REMOTE_PATH:-/home/${REMOTE_USER}/infra/lostdogs}"
IMAGE="${IMAGE:-jehaby/lostdogs:latest}"

: "${REMOTE_USER:?REMOTE_USER must be set in env file}"
: "${REMOTE_HOST:?REMOTE_HOST must be set in env file}"
: "${VK_TOKEN:?VK_TOKEN must be set in env file}"
: "${TG_TOKEN:?TG_TOKEN must be set in env file}"
: "${TG_CHAT:?TG_CHAT must be set in env file}"

IMAGE_FILENAME="lostdogs-image.tar"
REMOTE_ENV_FILE="${REMOTE_PATH%/}/.env"
REMOTE_COMPOSE_FILE="${REMOTE_PATH%/}/compose.yml"

echo "1) Building Docker image: ${IMAGE} for platform ${PLATFORM}"
# Use buildx to ensure correct platform (e.g., macOS -> linux/amd64)
if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required for cross-platform builds (install Docker Desktop / enable buildx)" >&2
  exit 2
fi
docker buildx build \
  --platform "${PLATFORM}" \
  --file "${DOCKERFILE}" \
  --load \
  -t "${IMAGE}" .

echo "2) Saving image -> ${IMAGE_FILENAME}"
docker save "${IMAGE}" -o "${IMAGE_FILENAME}"

echo "3) Ensuring remote dir exists: ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}"
ssh "${REMOTE_USER}@${REMOTE_HOST}" "mkdir -p \"${REMOTE_PATH}\""

echo "4) Copying files to remote: ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}/"
scp "${IMAGE_FILENAME}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}/"
scp "${TMP_ENV_FILE}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_ENV_FILE}"
# Ship config file with group list if present, canonicalize to config.yml on remote
if [[ -f "config.yaml" ]]; then
  scp "config.yaml" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH%/}/config.yml"
elif [[ -f "config.yml" ]]; then
  scp "config.yml" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH%/}/config.yml"
fi
scp "compose.yml" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_COMPOSE_FILE}"

echo "5) Running docker compose remotely"
ssh "${REMOTE_USER}@${REMOTE_HOST}" "REMOTE_PATH=${REMOTE_PATH}" bash -s <<'REMOTE_SCRIPT'
set -euo pipefail

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose plugin not available on remote host" >&2
  exit 2
fi

REMOTE_PATH="${REMOTE_PATH:-/tmp}"
REMOTE_ENV_FILE="${REMOTE_PATH%/}/.env"
REMOTE_COMPOSE_FILE="${REMOTE_PATH%/}/compose.yml"
IMAGE_FILENAME="${REMOTE_PATH%/}/lostdogs-image.tar"

# Always remove image tar on exit, even on failure before explicit cleanup
trap 'rm -f "${IMAGE_FILENAME}" || true' EXIT

echo "  a) Loading Docker image: ${IMAGE_FILENAME}"
docker load -i "${IMAGE_FILENAME}"

echo "  b) Compose up (force recreate to pick new image)"
# docker compose will auto-load environment from ${REMOTE_PATH}/.env
docker compose -f "${REMOTE_COMPOSE_FILE}" up -d --force-recreate --remove-orphans

echo "  c) Cleanup image tar (via trap)"
REMOTE_SCRIPT

echo "6) Cleanup local artifacts"
rm -f "${IMAGE_FILENAME}" "${TMP_ENV_FILE}"

echo "7) Done"
