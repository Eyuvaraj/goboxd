#!/usr/bin/env bash
# Load test for goboxd — POST /run with a trivial py3 hello-world.
# Requires: hey (https://github.com/rakyll/hey) or k6 (https://k6.io)
# Usage: bash scripts/load_test.sh [HOST]

set -euo pipefail

HOST="${1:-http://localhost:8080}"
ENDPOINT="${HOST}/run"

BODY='{
  "language": "py3",
  "source": "print(\"Hello, World!\")",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}'

if command -v hey &>/dev/null; then
  echo "=== Using hey ==="
  for C in 1 10 50 100; do
    echo ""
    echo "--- ${C} concurrent clients ---"
    echo "$BODY" | hey -n 200 -c "$C" -m POST \
      -T "application/json" \
      -D /dev/stdin \
      "$ENDPOINT"
  done
elif command -v k6 &>/dev/null; then
  echo "=== Using k6 ==="
  for C in 1 10 50 100; do
    echo ""
    echo "--- ${C} concurrent clients ---"
    k6 run --vus "$C" --iterations 200 - <<'K6EOF'
import http from 'k6/http';
import { check } from 'k6';

const body = JSON.stringify({
  language: 'py3',
  source: 'print("Hello, World!")',
  tests: [{ stdin: '', expected_stdout: 'Hello, World!\n' }],
});

export default function () {
  const res = http.post(__ENV.ENDPOINT, body, {
    headers: { 'Content-Type': 'application/json' },
  });
  check(res, { 'status 200': (r) => r.status === 200 });
}
K6EOF
  done
else
  echo "Error: install 'hey' or 'k6' to run load tests." >&2
  echo "  hey: go install github.com/rakyll/hey@latest" >&2
  echo "  k6:  https://k6.io/docs/get-started/installation/" >&2
  exit 1
fi
