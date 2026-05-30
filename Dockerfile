ARG GO_VERSION=1.26
ARG DEBIAN_VERSION=bookworm

# ---- Build nsjail 3.4 from source ----
# nsjail is included as a git submodule at external/nsjail (pinned to tag 3.4).
# Build it from the checked-out source rather than fetching from the network at build time.
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc libnl-route-3-dev \
        libprotobuf-dev libtool make pkg-config protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
COPY external/nsjail /src/nsjail
RUN make -C /src/nsjail \
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
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
        wget \
        # Language runtimes and compilers
        python3 \
        nodejs \
        gcc g++ \
        default-jdk-headless \
        iverilog \
        bash \
        ruby \
        lua5.4 \
        ocaml \
        # rustc \
        # kotlin \
    && rm -rf /var/lib/apt/lists/*

# Copy the Go toolchain from the builder stage (same version, avoids stale apt package).
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

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
    && bash --version \
    && ruby --version \
    && lua5.4 -e 'print("ok")' \
    && ocaml -version \
    # && rustc --version \
    # && kotlinc -version \
    && go version

RUN mkdir -p /tmp/goboxd

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd"]
