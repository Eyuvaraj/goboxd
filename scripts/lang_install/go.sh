#!/usr/bin/env bash
set -euo pipefail
apt-get install -y --no-install-recommends golang-go
go version
