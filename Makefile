.PHONY: test test-short test-integration test-api test-repo test-race lint build generate generate-go generate-ts tidy

test-short:
	go test -short ./...

test: test-short test-integration

test-integration: test-api test-repo

test-api:
	go test -tags=integration -timeout=10m ./tests/integration/api/...

test-repo:
	go test -tags=integration -timeout=10m ./internal/adapter/postgres/...

test-race:
	go test -race -tags=integration -timeout=15m ./tests/integration/api/... ./internal/adapter/postgres/...

lint:
	golangci-lint run

build:
	go build -o /tmp/api ./cmd/api/
	go build -o /tmp/scheduler ./cmd/scheduler/
	go build -o /tmp/worker ./cmd/worker/
	go build -o /tmp/notifier ./cmd/notifier/

# `make generate` regenerates every piece of checked-in generated code.
# Run after editing api/openapi.yaml or internal/sqlc/**.sql.
generate: generate-go generate-ts

generate-go:
	go generate ./tools/...

generate-ts:
	cd frontend && pnpm gen:types

tidy:
	go mod tidy
