.PHONY: build run test integration lint load

COMPOSE ?= docker compose
TOOLS   := $(COMPOSE) --profile tools run --rm tools

# Build the runtime Docker image.
build:
	$(COMPOSE) build goboxd

# Start the service locally (requires Docker).
run:
	$(COMPOSE) up goboxd

# Run unit tests (no nsjail required).
test:
	$(TOOLS) go test ./internal/... ./cmd/...

# Run integration tests (requires nsjail inside the container).
integration:
	$(TOOLS) go test -tags=integration -v -timeout=120s ./tests/...

# Run golangci-lint.
lint:
	$(TOOLS) golangci-lint run ./...

# Run the load test benchmark script (requires hey or k6 in PATH).
load:
	@bash scripts/load_test.sh
