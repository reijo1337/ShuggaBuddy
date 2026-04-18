.PHONY: run build migrate migrate-down lint fmt test test-e2e

GOLANGCI_LINT_VERSION ?= v2.11.3
GOLANGCI_LINT_DOCKER = docker run --rm \
	-v $(CURDIR):/app \
	-w /app \
	golangci/golangci-lint:$(GOLANGCI_LINT_VERSION)

NUM_CPU := $(shell getconf _NPROCESSORS_ONLN)

run:
	go run ./cmd/bot/main.go

build:
	go build -o bin/shuggabuddy ./cmd/bot/main.go

migrate:
	goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir migrations postgres "$$DATABASE_URL" down

lint:
	$(GOLANGCI_LINT_DOCKER) golangci-lint run ./...

fmt:
	$(GOLANGCI_LINT_DOCKER) golangci-lint fmt ./...

test:
	go test -p $(NUM_CPU) -v -race ./...

test-e2e:
	go test -v -tags e2e -count=1 ./tests/e2e/...
