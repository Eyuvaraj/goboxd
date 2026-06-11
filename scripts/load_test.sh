#!/usr/bin/env bash
# Load test for goboxd — POST /run with py3 and cpp hello-world payloads.
# Requires: hey (https://github.com/rakyll/hey) or k6 (https://k6.io)
# Usage: bash scripts/load_test.sh [HOST]

set -euo pipefail

# Add Go binary path to PATH if hey/k6 is not immediately available
if ! command -v hey &>/dev/null && ! command -v k6 &>/dev/null; then
  if command -v go &>/dev/null; then
    GOPATH_BIN="$(go env GOPATH)/bin"
    if [ -d "$GOPATH_BIN" ]; then
      export PATH="$GOPATH_BIN:$PATH"
    fi
  fi
  if [ -d "$HOME/go/bin" ]; then
    export PATH="$HOME/go/bin:$PATH"
  fi
fi

# Auto-install hey if still not found and Go is available
if ! command -v hey &>/dev/null && ! command -v k6 &>/dev/null; then
  if command -v go &>/dev/null; then
    echo "hey not found — installing via go install github.com/rakyll/hey@latest ..."
    go install github.com/rakyll/hey@latest
    export PATH="$(go env GOPATH)/bin:$PATH"
  fi
fi

HOST="${1:-http://localhost:8080}"
ENDPOINT="${HOST}/run"

PY3_BODY='{
  "language": "py3",
  "source": "print(\"Hello, World!\")",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}'

CPP_BODY='{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"Hello, World!\\n\";}",
  "source_filename": "solution.cpp",
  "artifact_filename": "solution",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}'

run_hey() {
  local label="$1"
  local body="$2"
  echo ""
  echo "=============================="
  echo " Language: ${label}"
  echo "=============================="
  for C in 1 10 50 100; do
    echo ""
    echo "--- ${C} concurrent clients ---"
    # cpp has a build step; use fewer requests so the run doesn't take too long
    local N=200
    [ "$label" = "cpp" ] && N=100
    printf '%s' "$body" | hey -n "$N" -c "$C" -m POST \
      -T "application/json" \
      -D /dev/stdin \
      "$ENDPOINT"
  done
}

run_k6() {
  local label="$1"
  local body="$2"
  echo ""
  echo "=============================="
  echo " Language: ${label}"
  echo "=============================="
  for C in 1 10 50 100; do
    local N=200
    [ "$label" = "cpp" ] && N=100
    echo ""
    echo "--- ${C} concurrent clients ---"
    ENDPOINT="$ENDPOINT" K6_BODY="$body" k6 run --vus "$C" --iterations "$N" - <<'K6EOF'
import http from 'k6/http';
import { check } from 'k6';
export default function () {
  const res = http.post(__ENV.ENDPOINT, __ENV.K6_BODY, {
    headers: { 'Content-Type': 'application/json' },
  });
  check(res, { 'status 200': (r) => r.status === 200 });
}
K6EOF
  done
}

if command -v hey &>/dev/null; then
  echo "=== Using hey ==="
  run_hey "py3" "$PY3_BODY"
  run_hey "cpp" "$CPP_BODY"
elif command -v k6 &>/dev/null; then
  echo "=== Using k6 ==="
  run_k6 "py3" "$PY3_BODY"
  run_k6 "cpp" "$CPP_BODY"
else
  echo "Error: install 'hey' or 'k6' to run load tests." >&2
  echo "  hey: go install github.com/rakyll/hey@latest" >&2
  echo "  k6:  https://k6.io/docs/get-started/installation/" >&2
  exit 1
fi
