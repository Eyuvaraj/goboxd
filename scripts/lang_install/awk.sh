#!/usr/bin/env bash
set -euo pipefail
# mawk is tiny and provides the default /usr/bin/awk in Debian; install guarantees it.
apt-get install -y --no-install-recommends mawk
awk 'BEGIN { print "awk ok" }'
