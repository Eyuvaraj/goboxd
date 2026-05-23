# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.23
ARG DEBIAN_VERSION=bookworm
ARG NSJAIL_VERSION=3.4

# ---- Build nsjail 3.4 from source ----
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder
ARG NSJAIL_VERSION
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc git libnl-route-3-dev \
        libprotobuf-dev libtool make pkg-config protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 --branch ${NSJAIL_VERSION} \
        https://github.com/google/nsjail.git /src/nsjail \
    && make -C /src/nsjail \
    && install -m 0755 /src/nsjail/nsjail /usr/local/bin/nsjail

# ---- Builder / dev image (Go + linters + nsjail) ----
FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
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
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
        # Language runtimes and compilers
        python3 \
        nodejs \
        gcc g++ \
        default-jdk-headless \
        iverilog \
        bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
COPY --from=builder        /out/goboxd           /usr/local/bin/goboxd
COPY configs/languages.yaml /etc/goboxd/languages.yaml

# Smoke-test each language at image-build time to catch missing toolchains early.
RUN python3 --version \
    && node --version \
    && gcc --version \
    && g++ --version \
    && java -version \
    && iverilog -V \
    && bash --version

RUN mkdir -p /tmp/goboxd

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd"]
