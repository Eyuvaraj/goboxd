#!/usr/bin/env bash
set -euo pipefail
# kotlin package in Debian bookworm provides /usr/bin/kotlinc.
# default-jdk-headless must already be installed for the JVM runtime.
apt-get install -y --no-install-recommends kotlin
kotlinc -version
