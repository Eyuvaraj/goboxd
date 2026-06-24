#!/usr/bin/env bash
# Kotlin disabled: Debian bookworm apt ships Kotlin 1.3.31 (2019).
# Modern Kotlin 2.x syntax fails to compile. Re-enable when a newer toolchain
# is available (e.g. download from JetBrains GitHub releases).
# set -euo pipefail
# apt-get install -y --no-install-recommends kotlin
# kotlinc -version
