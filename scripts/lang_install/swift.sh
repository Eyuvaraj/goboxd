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

/usr/local/swift/usr/bin/swiftc --version
