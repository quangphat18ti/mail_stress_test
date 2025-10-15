#!/usr/bin/env bash
set -euo pipefail

# Mail Stress Test helper script
# Works on macOS/Linux; handles spaces in paths.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_PATH="$ROOT_DIR/mail-stress-test"
DEFAULT_CONFIG="$ROOT_DIR/config/default.yaml"
REPORT_DIR="$ROOT_DIR/reports"

print_header() {
  echo "==== $1 ===="
}

usage() {
  cat <<'EOF'
Usage: ./run.sh <command> [options]

Commands:
  setup                 Install deps (go mod tidy) and create report dir
  build                 Build binary to ./mail-stress-test
  seed [-- -extra]      Seed initial data using -seed
  stress [-- -extra]    Run stress test only (-stress -benchmark=false)
  bench [-- -extra]     Run search benchmark only (-stress=false -benchmark)
  all [-- -extra]       Run both stress test and benchmark (default)
  open-report           Open the latest generated HTML chart (macOS 'open')
  clean                 Remove binary and generated reports

Options:
  -c, --config <path>   Path to config YAML (default: config/default.yaml)
  -d, --docker          Run commands via Docker Compose instead of locally

Tips:
  Append flags after "--" to pass directly to the Go program (e.g., duration via config).
  Environment overrides: MONGO_URI, MONGO_DATABASE, CONFIG_PATH
EOF
}

ensure_go() {
  if ! command -v go >/dev/null 2>&1; then
    echo "Error: Go is not installed or not in PATH." >&2
    exit 1
  fi
}

setup() {
  if [[ "$USE_DOCKER" == true ]]; then
    echo "Setup not needed in Docker mode. Dependencies are handled in Dockerfile."
    return
  fi
  ensure_go
  print_header "Downloading dependencies"
  (cd "$ROOT_DIR" && go mod tidy)
  mkdir -p "$REPORT_DIR"
  echo "Done."
}

build() {
  if [[ "$USE_DOCKER" == true ]]; then
    echo "Build not needed in Docker mode. Use 'docker-compose build' instead."
    return
  fi
  ensure_go
  print_header "Building binary"
  (cd "$ROOT_DIR" && go build -o "$BINARY_PATH" ./cmd/main.go)
  echo "Built: $BINARY_PATH"
}

run_program() {
  local config_path="$1"; shift || true

  # Normalize relative config path to absolute under project root
  if [[ "$config_path" != /* ]]; then
    config_path="$ROOT_DIR/$config_path"
  fi
  [[ -f "$config_path" ]] || { echo "Config not found: $config_path" >&2; exit 1; }

  if [[ "$USE_DOCKER" == true ]]; then
    # Ensure docker-compose is available
    if ! command -v docker-compose >/dev/null 2>&1; then
      echo "Error: docker-compose not found. Install Docker Compose to use --docker." >&2
      exit 1
    fi
    print_header "Running via Docker Compose: $(basename "$BINARY_PATH")"
    # In Docker, config is always at config/default.yaml (copied in Dockerfile)
    docker-compose run --rm app ./mail-stress-test -config=config/default.yaml "$@"
  else
    [[ -x "$BINARY_PATH" ]] || build
    print_header "Running: $(basename "$BINARY_PATH")"
    "$BINARY_PATH" -config="$config_path" "$@"
  fi
}

seed_cmd() {
  local config_path="$1"; shift || true
  run_program "$config_path" -seed "$@"
}

stress_cmd() {
  local config_path="$1"; shift || true
  run_program "$config_path" -stress -benchmark=false "$@"
}

bench_cmd() {
  local config_path="$1"; shift || true
  run_program "$config_path" -stress=false -benchmark "$@"
}

all_cmd() {
  local config_path="$1"; shift || true
  run_program "$config_path" "$@"
}

open_report() {
  local latest
  latest=$(ls -t "$REPORT_DIR"/charts_*.html 2>/dev/null | head -n 1 || true)
  if [[ -z "${latest:-}" ]]; then
    echo "No chart HTML found in $REPORT_DIR. Run a test with charts enabled."
    exit 1
  fi
  print_header "Opening: $latest"
  if command -v open >/dev/null 2>&1; then
    open "$latest"
  else
    echo "Open this file in your browser: $latest"
  fi
}

clean_cmd() {
  print_header "Cleaning outputs"
  rm -f "$BINARY_PATH"
  rm -rf "$REPORT_DIR"
  echo "Cleaned."
}

# --- entrypoint ---
CMD="${1:-}"
shift || true

CONFIG="$DEFAULT_CONFIG"
EXTRA_ARGS=()
USE_DOCKER=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    -c|--config)
      CONFIG="${2:-}"
      shift 2
      ;;
    -d|--docker)
      USE_DOCKER=true
      shift
      ;;
    --)
      shift
      # Remainder passed as-is
      while [[ $# -gt 0 ]]; do EXTRA_ARGS+=("$1"); shift; done
      break
      ;;
    *)
      EXTRA_ARGS+=("$1")
      shift
      ;;
  esac
done

case "$CMD" in
  setup)
    setup ;;
  build)
    build ;;
  seed)
    seed_cmd "$CONFIG" "${EXTRA_ARGS[@]:-}" ;;
  stress)
    stress_cmd "$CONFIG" "${EXTRA_ARGS[@]:-}" ;;
  bench|benchmark)
    bench_cmd "$CONFIG" "${EXTRA_ARGS[@]:-}" ;;
  all|run|start)
    all_cmd "$CONFIG" "${EXTRA_ARGS[@]:-}" ;;
  open-report)
    open_report ;;
  clean)
    clean_cmd ;;
  -h|--help|help|"")
    usage ;;
  *)
    echo "Unknown command: $CMD" >&2
    echo
    usage
    exit 1
    ;;
esac
