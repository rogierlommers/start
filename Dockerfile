# syntax=docker/dockerfile:1

FROM golang:1.26.2-alpine AS builder

WORKDIR /src

ARG VERSION

# Cache module downloads first.
COPY go.mod go.sum ./
RUN go mod download

# Build application.
COPY . .
RUN APP_BUILD_TIME="$(TZ=Europe/Amsterdam date +%Y-%m-%d\ %H:%M:%S\ %Z)" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'main.appBuildTime=${APP_BUILD_TIME}'" -o /out/start ./cmd/start

FROM alpine:3.20

RUN addgroup -S app && adduser -S -G app app && \
    apk add --no-cache ca-certificates && \
    mkdir -p /app/uploads && \
    chown -R app:app /app

WORKDIR /app

# Keep docs available for /docs endpoint (server reads ./docs/swagger.yaml).
COPY --from=builder /out/start /app/start
COPY --from=builder /src/docs /app/docs

# Config loader currently expects a .env file to exist.
RUN touch /app/.env && chown app:app /app/.env

ENV HTTP_BIND_ADDR=0.0.0.0:3000
ENV STORAGE_UPLOAD_DIR=uploads

EXPOSE 3000

USER app

ENTRYPOINT ["/app/start"]
