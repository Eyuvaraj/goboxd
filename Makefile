.PHONY: build run test integration lint load swagger

COMPOSE ?= docker compose
TOOLS   := $(COMPOSE) --profile tools run --rm tools
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

# Build the runtime Docker image.
build:
	git submodule update --init --recursive
	$(COMPOSE) build --build-arg COMMIT=$(COMMIT) goboxd

# Start the service locally (requires Docker).
run:
	$(COMPOSE) up goboxd

# Run unit tests inside the tools container (no nsjail required).
# --no-deps skips starting goboxd so this works without make run.
test:
	$(COMPOSE) --profile tools run --rm --no-deps tools go test ./internal/... ./cmd/...

# Run integration tests (requires nsjail inside the container).
integration:
	$(TOOLS) go test -tags=integration -v -timeout=120s ./tests/...

# Run golangci-lint.
lint:
	$(TOOLS) golangci-lint run ./...

# Run the Stage-3 load test (requires vegeta + jq in PATH, goboxd running on :8080).
load:
	@bash docs/loadtest/load-test.sh docs/loadtest

# Regenerate Swagger docs from annotations (runs inside the tools container).
swagger:
	$(TOOLS) sh -c 'which swag > /dev/null 2>&1 || go install github.com/swaggo/swag/cmd/swag@latest && swag init -g cmd/goboxd/main.go --output docs --parseInternal'
