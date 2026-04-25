default:
	@just --list

run: generate-openapi
	go run ./cmd/start

watch:
	@if ! command -v air >/dev/null 2>&1; then \
		echo "Error: 'air' CLI is not installed. Install it with: go install github.com/air-verse/air@latest"; \
		exit 1; \
	fi
	air

test:
	go test ./...

generate-openapi:
	go run github.com/swaggo/swag/cmd/swag@v1.16.4 init --generalInfo main.go --dir cmd/start,internal/httpapi,internal/httpweb --output docs --outputTypes yaml --parseInternal --generatedTime=false
