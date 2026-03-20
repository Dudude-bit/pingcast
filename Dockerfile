FROM golang:1.25-alpine AS builder
ARG SERVICE
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /service ./cmd/${SERVICE}/

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /service /service
CMD ["/service"]
