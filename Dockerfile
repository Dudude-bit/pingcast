FROM golang:1.26-alpine AS builder
ARG SERVICE
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/kirillinakin/pingcast/internal/version.Version=${VERSION} -X github.com/kirillinakin/pingcast/internal/version.Commit=${COMMIT}" \
    -o /service ./cmd/${SERVICE}/

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /service /service
USER nobody
ENTRYPOINT ["/service"]
