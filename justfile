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

generate-openapi:
	go run github.com/swaggo/swag/cmd/swag@v1.16.4 init --generalInfo main.go --dir cmd/start,internal/httpapi,internal/httpweb --output docs --outputTypes yaml --parseInternal --generatedTime=false

seed-bookmarks base_url="http://localhost:3000":
	curl http://0.0.0.0:3000/api/categories --request POST --header 'Content-Type: application/json' --data '{"name": "category1"}'
	curl http://0.0.0.0:3000/api/categories --request POST --header 'Content-Type: application/json' --data '{"name": "category2"}'
	curl http://0.0.0.0:3000/api/categories --request POST --header 'Content-Type: application/json' --data '{"name": "category3"}'
	curl http://0.0.0.0:3000/api/categories --request POST --header 'Content-Type: application/json' --data '{"name": "category4"}'

	# category 1
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 1,"title": "bookmark1","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 1,"title": "bookmark2","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 1,"title": "bookmark3","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 1,"title": "bookmark4","url": "https://just-a-bookmar.com"}'

	# category 2
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 2,"title": "bookmark1","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 2,"title": "bookmark2","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 2,"title": "bookmark3","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 2,"title": "bookmark4","url": "https://just-a-bookmar.com"}'

	# category 3
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 3,"title": "bookmark1","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 3,"title": "bookmark2","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 3,"title": "bookmark3","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 3,"title": "bookmark4","url": "https://just-a-bookmar.com"}'

	# category 4
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 4,"title": "bookmark1","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 4,"title": "bookmark2","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 4,"title": "bookmark3","url": "https://just-a-bookmar.com"}'
	curl http://0.0.0.0:3000/api/bookmarks --request POST --header 'Content-Type: application/json' --data '{"category_id": 4,"title": "bookmark4","url": "https://just-a-bookmar.com"}'
