#!/usr/bin/env bash
set -euo pipefail
ZIG_VERSION=0.13.0
apt-get install -y --no-install-recommends curl xz-utils
curl -fsSL "https://ziglang.org/download/${ZIG_VERSION}/zig-linux-x86_64-${ZIG_VERSION}.tar.xz" \
  | tar -xJ -C /usr/local/
ln -sf "/usr/local/zig-linux-x86_64-${ZIG_VERSION}/zig" /usr/local/bin/zig
zig version
