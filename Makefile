.PHONY: build run up down logs test test-local integration lint load clean swagger

COMPOSE ?= docker compose
TOOLS   := $(COMPOSE) --profile tools run --rm tools
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
# Use the exact git tag if on one; otherwise fall back to the short commit hash.
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo dev)

# Build the runtime Docker image.
build:
	git submodule update --init --recursive
	$(COMPOSE) build \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg VERSION=$(VERSION) \
		goboxd

# Start the service in the foreground (Ctrl-C to stop).
run:
	$(COMPOSE) up goboxd

# Start the service detached — for VM/production use.
up:
	$(COMPOSE) up -d goboxd

# Stop and remove the service container.
down:
	$(COMPOSE) down

# Tail service logs.
logs:
	$(COMPOSE) logs -f goboxd

# Run unit tests inside the tools container (no nsjail required).
test:
	$(COMPOSE) --profile tools run --rm --no-deps tools go test ./internal/... ./cmd/...

# Run unit tests without Docker (requires a local Go toolchain).
test-local:
	go test ./internal/... ./cmd/...

# Run integration tests (requires nsjail; starts goboxd automatically if not running).
integration:
	$(TOOLS) go test -tags=integration -v -timeout=120s ./tests/...

# Run golangci-lint.
lint:
	$(TOOLS) golangci-lint run ./...

load:
	@bash scripts/load_test.sh

# Remove built images, stopped containers, and anonymous volumes.
clean:
	$(COMPOSE) down --rmi local --volumes --remove-orphans

# Regenerate Swagger docs from annotations (runs inside the tools container).
swagger:
	$(TOOLS) sh -c 'which swag > /dev/null 2>&1 || go install github.com/swaggo/swag/cmd/swag@latest && swag init -g cmd/goboxd/main.go --output docs --parseInternal'
