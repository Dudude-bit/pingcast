package tools

//go:generate sqlc generate -f ../internal/sqlc/sqlc.yaml
//go:generate oapi-codegen -config ../internal/api/oapi-config.yaml ../api/openapi.yaml
