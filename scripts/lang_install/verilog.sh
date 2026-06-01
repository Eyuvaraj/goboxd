#!/usr/bin/env bash
set -euo pipefail
apt-get update --allow-releaseinfo-change
apt-get install -y --no-install-recommends iverilog
iverilog -V
vvp -v 2>&1 | head -1 || true
