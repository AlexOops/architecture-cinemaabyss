#!/bin/bash
set -e

# Script to run Postman tests using Newman

function show_usage {
  echo "Usage: $0 [options]"
  echo ""
  echo "Options:"
  echo "  -e, --environment ENV   Specify environment (local, docker)"
  echo "  -f, --folder FOLDER     Run specific test folder"
  echo "  -r, --reporters LIST    Comma-separated list of reporters"
  echo "  -b, --bail              Stop on first error"
  echo "  -t, --timeout MS        Request timeout in milliseconds"
  echo "  -d, --docker            Run tests in Docker container"
  echo "  -h, --help              Show this help message"
}

ENVIRONMENT="local"
FOLDER=""
REPORTERS="cli,htmlextra,junit"
BAIL=false
TIMEOUT=10000
USE_DOCKER=false

while [[ $# -gt 0 ]]; do
  case $1 in
    -e|--environment) ENVIRONMENT="$2"; shift 2 ;;
    -f|--folder) FOLDER="$2"; shift 2 ;;
    -r|--reporters) REPORTERS="$2"; shift 2 ;;
    -b|--bail) BAIL=true; shift ;;
    -t|--timeout) TIMEOUT="$2"; shift 2 ;;
    -d|--docker) USE_DOCKER=true; shift ;;
    -h|--help) show_usage; exit 0 ;;
    *) echo "Unknown option: $1"; show_usage; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ "$USE_DOCKER" = false ]; then
  command -v node >/dev/null 2>&1 || { echo "Error: Node.js is not installed."; exit 1; }
  command -v npm  >/dev/null 2>&1 || { echo "Error: npm is not installed."; exit 1; }
fi

# Build args as array (IMPORTANT)
CMD_ARGS=(--environment "$ENVIRONMENT")

if [ -n "$FOLDER" ]; then
  CMD_ARGS+=(--folder "$FOLDER")
fi

if [ -n "$REPORTERS" ]; then
  CMD_ARGS+=(--reporters "$REPORTERS")
fi

if [ "$BAIL" = true ]; then
  CMD_ARGS+=(--bail)
fi

if [ -n "$TIMEOUT" ]; then
  CMD_ARGS+=(--timeout "$TIMEOUT")
fi

mkdir -p reports

if [ "$USE_DOCKER" = true ]; then
  echo "Running tests in Docker container..."

  docker build -t cinemaabyss-api-tests .

  # fixed network name (matches docker-compose.yml: networks.cinemaabyss-network.name)
  NETWORK_NAME="${COMPOSE_NETWORK:-cinemaabyss-network}"

  echo "Using Docker network: $NETWORK_NAME"

  docker run --rm \
    --network="$NETWORK_NAME" \
    -v "$(pwd)/reports:/app/reports" \
    cinemaabyss-api-tests "${CMD_ARGS[@]}"

else
  echo "Running tests locally..."

  if [ ! -d "node_modules" ]; then
    npm install
  fi

  node run-tests.js "${CMD_ARGS[@]}"
fi