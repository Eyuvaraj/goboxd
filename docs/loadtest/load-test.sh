#!/usr/bin/env bash
# load-test.sh — Stage 3 load test for goboxd using MemoryHog.java
#
# Usage: ./docs/loadtest/load-test.sh [output_dir]
#
# Requirements:
#   - vegeta  (brew install vegeta)
#   - jq      (brew install jq)
#   - Docker  (goboxd container must be running on port 8080)
#
# Container limits:   2 vCPU, 2 GB RAM  (set in docker-compose.override.yml)
# Per-request timeout: 10 s
# Steps (req/s):       5 10 25 50 75 100 150 200 300 400

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${1:-${SCRIPT_DIR}}"
REQUEST_FILE="${SCRIPT_DIR}/run-request.json"
TIMEOUT="10s"
DURATION="30s"

# Generate a portable target.txt pointing at the resolved request file.
# vegeta's @file syntax requires an absolute path; we write it here so the
# file works from any clone location without manual editing.
TARGET_FILE="${OUTPUT_DIR}/target.txt"
printf 'POST http://localhost:8080/run\nContent-Type: application/json\n@%s\n' \
    "${REQUEST_FILE}" > "${TARGET_FILE}"

# Load steps — keep climbing until failures appear, then two more steps
RATES=(5 10 25 50 75 100 150 200 300 400)

echo "=== goboxd Stage-3 Load Test ==="
echo "Output dir  : ${OUTPUT_DIR}"
echo "Target file : ${TARGET_FILE}"
echo "Timeout     : ${TIMEOUT}"
echo "Duration    : ${DURATION} per step"
echo "Steps       : ${RATES[*]}"
echo ""

# ---- Sanity checks ------------------------------------------------
if ! command -v vegeta &>/dev/null; then
    echo "ERROR: vegeta not found. Install with: brew install vegeta"
    exit 1
fi
if ! command -v jq &>/dev/null; then
    echo "ERROR: jq not found. Install with: brew install jq"
    exit 1
fi
if ! curl -sf http://localhost:8080/healthz | grep -q '"status":"ok"'; then
    echo "ERROR: goboxd is not responding on http://localhost:8080"
    echo "Start it with: docker compose up -d goboxd"
    exit 1
fi

echo "✓ Pre-flight checks passed"
echo ""

mkdir -p "${OUTPUT_DIR}"

# ---- CSV header ---------------------------------------------------
CSV_FILE="${OUTPUT_DIR}/results.csv"
echo "target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms" > "${CSV_FILE}"

# ---- Run each step ------------------------------------------------
BROKE_AT=""
for rate in "${RATES[@]}"; do
    REPORT_FILE="${OUTPUT_DIR}/report-${rate}.json"

    echo "→ Rate: ${rate} req/s for ${DURATION} ..."
    vegeta attack \
        -rate="${rate}/1s" \
        -duration="${DURATION}" \
        -timeout="${TIMEOUT}" \
        -targets="${TARGET_FILE}" \
        -max-body=0 \
        | vegeta report -type=json > "${REPORT_FILE}"

    # Extract metrics and append CSV row
    jq -r --arg r "${rate}" '
        .status_codes as $codes |
        ($codes["200"] // 0 | tonumber) as $ok |
        (.requests - $ok) as $fail |
        [
          $r,
          (.throughput | . * 100 | round / 100),
          (.duration / 1e9 | . * 100 | round / 100),
          .requests,
          $ok,
          $fail,
          (if .requests > 0 then ($fail / .requests * 100 | . * 100 | round / 100) else 0 end),
          (.latencies["50th"] / 1e6 | . * 100 | round / 100),
          (.latencies["95th"] / 1e6 | . * 100 | round / 100),
          (.latencies["99th"] / 1e6 | . * 100 | round / 100),
          (.latencies.max / 1e6 | . * 100 | round / 100)
        ] | @csv
    ' "${REPORT_FILE}" >> "${CSV_FILE}"

    # Check if this step had failures — print a quick summary.
    # Use floor+round in jq to guarantee an integer string (no trailing .0)
    # so bash's arithmetic comparison does not fail on floats.
    FAIL_COUNT=$(jq -r '(.requests - (.status_codes["200"] // 0 | tonumber)) | floor | round' "${REPORT_FILE}")
    ERR_PCT=$(jq -r '
        (.requests - (.status_codes["200"] // 0 | tonumber)) as $f |
        if .requests > 0 then ($f / .requests * 100 | . * 100 | round / 100) else 0 end
    ' "${REPORT_FILE}")

    echo "   requests=$(jq .requests "${REPORT_FILE}") success=$(jq '.status_codes["200"] // 0' "${REPORT_FILE}") failed=${FAIL_COUNT} error=${ERR_PCT}%"
    echo "   p50=$(jq '(.latencies["50th"]/1e6|.*100|round/100)' "${REPORT_FILE}")ms  p95=$(jq '(.latencies["95th"]/1e6|.*100|round/100)' "${REPORT_FILE}")ms  p99=$(jq '(.latencies["99th"]/1e6|.*100|round/100)' "${REPORT_FILE}")ms"
    echo ""

    # Arithmetic expansion handles integer comparison safely regardless of jq version.
    if [[ -z "${BROKE_AT}" ]] && (( FAIL_COUNT > 0 )); then
        BROKE_AT="${rate}"
        echo "*** BREAKING POINT REACHED at ${rate} req/s ***"
        echo ""
    fi
done

echo "=== Test complete ==="
echo "CSV written to: ${CSV_FILE}"
if [[ -n "${BROKE_AT}" ]]; then
    echo "Breaking point: ${BROKE_AT} req/s"
else
    echo "Breaking point: NOT REACHED (service survived all steps)"
fi
echo ""
echo "Next step: run the plot script:"
echo "  python3 ${SCRIPT_DIR}/plot.py ${CSV_FILE} ${OUTPUT_DIR}"
