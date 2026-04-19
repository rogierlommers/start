default:
	@just --list

run:
	go run ./cmd/start

generate-openapi:
	go run github.com/swaggo/swag/cmd/swag@v1.16.4 init --generalInfo cmd/start/main.go --dir cmd/start,internal/httpapi,internal/httpweb --output docs --outputTypes yaml --parseInternal --generatedTime=false
