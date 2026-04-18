.PHONY: test test-short test-integration test-api test-repo lint build

test-short:
	go test -short ./...

test: test-short test-integration

test-integration: test-api test-repo

test-api:
	go test -tags=integration -timeout=10m ./tests/integration/api/...

test-repo:
	go test -tags=integration -timeout=10m ./internal/adapter/postgres/...

lint:
	golangci-lint run

build:
	go build -o /tmp/api ./cmd/api/
	go build -o /tmp/scheduler ./cmd/scheduler/
	go build -o /tmp/worker ./cmd/worker/
	go build -o /tmp/notifier ./cmd/notifier/
