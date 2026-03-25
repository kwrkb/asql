#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

export GOCACHE="${GOCACHE:-/tmp/asql-gocache}"
E2E_CONFIG_HOME="/tmp/asql-e2e-config"

# --- Prereqs ---
if ! command -v vhs &>/dev/null; then
  echo "SKIP: vhs not found — install with 'go install github.com/charmbracelet/vhs@latest'"
  exit 0
fi

# --- Build ---
echo "==> Building asql..."
go build -o ./asql .

# --- Prepare test DB ---
echo "==> Creating test databases..."
python3 docs/setup-demo-db.py /tmp/asql-e2e.db /tmp/asql-e2e-staging.db

# --- Prepare isolated config/profiles ---
echo "==> Creating test profiles..."
rm -rf "$E2E_CONFIG_HOME"
python3 e2e/setup-profiles.py "$E2E_CONFIG_HOME" /tmp/asql-e2e.db /tmp/asql-e2e-staging.db

# --- Prepare recordings dir ---
mkdir -p e2e/recordings

# --- Run tapes ---
tapes=(e2e/[0-9]*.tape)
passed=0
failed=0
failed_names=()

for tape in "${tapes[@]}"; do
  name=$(basename "$tape" .tape)
  echo -n "  $name ... "
  if vhs "$tape" 2>/tmp/asql-e2e-"$name".log; then
    echo "PASS"
    passed=$((passed + 1))
  else
    echo "FAIL"
    failed=$((failed + 1))
    failed_names+=("$name")
  fi
done

# --- Cleanup ---
rm -f /tmp/asql-e2e.db /tmp/asql-e2e-staging.db
rm -rf "$E2E_CONFIG_HOME"

# --- Summary ---
echo ""
echo "==> Results: $passed passed, $failed failed (total ${#tapes[@]})"
echo "  Recordings: e2e/recordings/"
if ((failed > 0)); then
  echo "  Failed:"
  for name in "${failed_names[@]}"; do
    echo "    - $name"
    echo "      Log: /tmp/asql-e2e-${name}.log"
  done
  exit 1
fi
