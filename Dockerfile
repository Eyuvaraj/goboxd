ARG GO_VERSION=1.26
ARG DEBIAN_VERSION=bookworm
# Must match the external/nsjail submodule tag; surfaced at runtime in /readyz
# and /info because nsjail itself exposes no --version flag.
ARG NSJAIL_VERSION=3.4

# ---- Build nsjail 3.4 from source ----
# nsjail is included as a git submodule at external/nsjail (pinned to tag 3.4).
# Build it from the checked-out source rather than fetching from the network at build time.
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc git libnl-route-3-dev \
        libprotobuf-dev libtool make pkg-config protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
COPY external/nsjail /src/nsjail
RUN make -j$(nproc) -C /src/nsjail \
    && install -m 0755 /src/nsjail/nsjail /usr/local/bin/nsjail

# ---- Builder / dev image (Go + linters + nsjail) ----
FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
WORKDIR /src
# Copy module files first for layer caching — deps rarely change.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Inject build metadata via ldflags.
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w -X github.com/thesouldev/goboxd/internal/handler.Version=${VERSION} \
                       -X github.com/thesouldev/goboxd/internal/handler.Commit=${COMMIT}" \
        -o /out/goboxd ./cmd/goboxd

# ---- Runtime image ----
FROM debian:${DEBIAN_VERSION}-slim AS runtime
# Install base runtime deps and all language toolchains in two layers:
# 1. Base deps that don't change often (separate from language toolchains for cache efficiency).
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
        bash \
    && rm -rf /var/lib/apt/lists/*

# 2. Language toolchains: each install script does its own apt-get install so
#    adding a new language is one script file with no Dockerfile edits needed.
#    Cache mounts persist apt lists across builds; no rm -rf needed here.
COPY scripts/lang_install /install
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && for s in /install/*.sh; do bash "$s"; done

COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
COPY --from=builder        /out/goboxd           /usr/local/bin/goboxd
COPY configs/languages.yaml /etc/goboxd/languages.yaml

# Copy the Go toolchain from the builder stage (same version, avoids stale apt package).
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# nsjail has no --version flag, so the probe reads its version from here.
ARG NSJAIL_VERSION=3.4
ENV NSJAIL_VERSION=${NSJAIL_VERSION}

# Language smoke tests are performed by their respective install scripts.

RUN mkdir -p /tmp/goboxd

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd"]
