#!/usr/bin/env bash
set -euo pipefail
apt-get update --allow-releaseinfo-change
apt-get install -y --no-install-recommends lua5.4
lua5.4 -e 'print("ok")'
