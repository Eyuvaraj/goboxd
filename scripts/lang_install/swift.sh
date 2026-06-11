#!/usr/bin/env bash
set -euo pipefail

SWIFT_VERSION=6.3.2
SWIFT_PLATFORM=debian12
SWIFT_URL="https://download.swift.org/swift-${SWIFT_VERSION}-release/${SWIFT_PLATFORM}/swift-${SWIFT_VERSION}-RELEASE/swift-${SWIFT_VERSION}-RELEASE-${SWIFT_PLATFORM}.tar.gz"

apt-get install -y --no-install-recommends \
  binutils-gold \
  curl \
  libcurl4-openssl-dev \
  libedit-dev \
  libicu-dev \
  libncurses-dev \
  libsqlite3-dev \
  libxml2-dev \
  tzdata \
  uuid-dev

mkdir -p /usr/local/swift
curl -fsSL "${SWIFT_URL}" | tar -xz --strip-components=1 -C /usr/local/swift

# Remove tools not needed for compilation: debugger, LSP, SPM, docs.
# Only swiftc + swift-frontend + clang + stdlib are required to compile and
# statically link Swift source files.
rm -rf \
  /usr/local/swift/usr/share \
  /usr/local/swift/usr/bin/lldb \
  /usr/local/swift/usr/bin/lldb-server \
  /usr/local/swift/usr/bin/sourcekit-lsp \
  /usr/local/swift/usr/bin/swift-package \
  /usr/local/swift/usr/bin/swift-build \
  /usr/local/swift/usr/bin/swift-test \
  /usr/local/swift/usr/bin/swift-run \
  /usr/local/swift/usr/bin/swift-docc \
  /usr/local/swift/usr/bin/swift-inspect \
  /usr/local/swift/usr/bin/swift-symbolgraph-extract \
  /usr/local/swift/usr/bin/swift-api-digester \
  /usr/local/swift/usr/bin/swift-api-checker \
  /usr/local/swift/usr/bin/swift-refactor \
  /usr/local/swift/usr/bin/swift-stdlib-tool

# Remove unused shared libraries from the lib directory.
# libclang.so  = Clang C API (IDE bindings) — not used by swiftc.
# libIndexStore = source indexing for SourceKit — not needed.
# libswiftDemangle / libswiftRemoteMirror = debugging tools — not needed.
# libLLVM / libclang-cpp are kept: swift-frontend links against them.
rm -f \
  /usr/local/swift/usr/lib/libclang.so* \
  /usr/local/swift/usr/lib/libIndexStore.so* \
  /usr/local/swift/usr/lib/libswiftDemangle.so* \
  /usr/local/swift/usr/lib/libswiftRemoteMirror.so*

/usr/local/swift/usr/bin/swiftc --version
