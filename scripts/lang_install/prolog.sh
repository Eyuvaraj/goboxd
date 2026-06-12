#!/usr/bin/env bash
set -euo pipefail

apt-get install -y --no-install-recommends swi-prolog-nox

swipl --version
