#!/usr/bin/env bash
set -euo pipefail
apt-get update --allow-releaseinfo-change
apt-get install -y --no-install-recommends ruby
ruby --version
