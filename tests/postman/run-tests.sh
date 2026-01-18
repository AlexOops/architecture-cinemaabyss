#!/bin/bash
set -euo pipefail

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
  echo ""
  echo "Examples:"
  echo "  $0 -e local"
  echo "  $0 -e docker -f \"Movies Microservice\""
  echo "  $0 -d -e docker"
}

# Default values
ENVIRONMENT="local"
FOLDER=""
REPORTERS="cli,htmlextra,junit"
BAIL=false
TIMEOUT=10000
USE_DOCKER=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -e|--environment)
      ENVIRONMENT="${2:-}"
      shift 2
      ;;
    -f|--folder)
      FOLDER="${2:-}"
      shift 2
      ;;
    -r|--reporters)
      REPORTERS="${2:-}"
      shift 2
      ;;
    -b|--bail)
      BAIL=true
      shift
      ;;
    -t|--timeout)
      TIMEOUT="${2:-}"
      shift 2
      ;;
    -d|--docker)
      USE_DOCKER=true
      shift
      ;;
    -h|--help)
      show_usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      show_usage
      exit 1
      ;;
  esac
done

# Ensure we're in the right directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check if Node.js and npm are installed if not using Docker
if [[ "$USE_DOCKER" == false ]]; then
  command -v node >/dev/null 2>&1 || { echo "Error: Node.js is not installed. Use -d to run in Docker."; exit 1; }
  command -v npm  >/dev/null 2>&1 || { echo "Error: npm is not installed. Use -d to run in Docker."; exit 1; }
fi

# Build command arguments as ARRAY (important)
CMD_ARGS=(--environment "$ENVIRONMENT")

if [[ -n "$FOLDER" ]]; then
  CMD_ARGS+=(--folder "$FOLDER")
fi

if [[ -n "$REPORTERS" ]]; then
  CMD_ARGS+=(--reporters "$REPORTERS")
fi

if [[ "$BAIL" == true ]]; then
  CMD_ARGS+=(--bail)
fi

if [[ -n "$TIMEOUT" ]]; then
  CMD_ARGS+=(--timeout "$TIMEOUT")
fi

# Create reports directory if it doesn't exist
mkdir -p reports

if [[ "$USE_DOCKER" == true ]]; then
  echo "Running tests in Docker container..."

  # Build the Docker image
  docker build -t cinemaabyss-api-tests .

  # Prefer fixed network name (matches docker-compose.yml with networks.cinemaabyss-network.name)
  NETWORK_NAME="${COMPOSE_NETWORK:-cinemaabyss-network}"

  # If the preferred network does not exist, try to detect from running compose containers
  if ! docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
    COMPOSE_CONTAINER_ID="$(docker compose ps -q | head -n 1 || true)"
    if [[ -z "$COMPOSE_CONTAINER_ID" ]]; then
      echo "❌ No running docker-compose containers found. Did you run 'docker compose up -d'?"
      exit 1
    fi

    NETWORK_NAME="$(docker inspect "$COMPOSE_CONTAINER_ID" \
      --format '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' \
      | head -n 1)"

    if [[ -z "$NETWORK_NAME" ]]; then
      echo "❌ Failed to detect docker network from container: $COMPOSE_CONTAINER_ID"
      exit 1
    fi
  fi

  echo "Using Docker network: $NETWORK_NAME"

  docker run --rm \
    --network="$NETWORK_NAME" \
    -v "$(pwd)/reports:/app/reports" \
    cinemaabyss-api-tests "${CMD_ARGS[@]}"

else
  echo "Running tests locally..."

  if [[ ! -d "node_modules" ]]; then
    echo "Installing dependencies..."
    npm install
  fi

  node run-tests.js "${CMD_ARGS[@]}"
fi

EXIT_CODE=$?

if [[ $EXIT_CODE -eq 0 ]]; then
  echo "✅ All tests passed!"
else
  echo "❌ Some tests failed. Check the reports for details."
fi

exit $EXIT_CODE