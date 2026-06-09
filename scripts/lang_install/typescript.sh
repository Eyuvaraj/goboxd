#!/usr/bin/env bash
set -euo pipefail
# node-typescript provides /usr/bin/tsc; it runs on the Node already installed by node.sh.
apt-get install -y --no-install-recommends node-typescript
tsc --version
