#!/usr/bin/env bash
set -euo pipefail
apt-get install -y --no-install-recommends mono-mcs mono-runtime
mcs --version
