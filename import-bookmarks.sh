#!/usr/bin/env bash
set -euo pipefail

BASE_URL="http://localhost:3000"
API_USERNAME=""
API_PASSWORD=""

create_category() {
	local name="$1"
	curl "${BASE_URL}/api/categories" --request POST \
		-u "${API_USERNAME}:${API_PASSWORD}" \
		--header "Content-Type: application/json" \
		--data "{\"name\":\"${name}\"}"
}

create_bookmark() {
	local title="$1"
	local url="$2"
	local category_id="$3"
	curl "${BASE_URL}/api/bookmarks" --request POST \
		-u "${API_USERNAME}:${API_PASSWORD}" \
		--header "Content-Type: application/json" \
		--data "{\"category_id\":${category_id},\"title\":\"${title}\",\"url\":\"${url}\"}"
}

# create categories
create_category "Personal"      # id 1
create_category "Home network"  # id 2
create_category "Fun"           # id 3
create_category "Work"          # id 4

# create bookmarks
create_bookmark "google" "https://www.google.com" 1
