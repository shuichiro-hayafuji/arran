.PHONY: start run test fmt build db-schema docker-build up down

GO ?= go
IMAGE ?= spendable-today-api
DATABASE_URL ?= postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable

start:
	./scripts/start-server.sh

run:
	$(GO) run ./cmd/api

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/spendable-today ./cmd/api

db-schema:
	psql "$(DATABASE_URL)" -c '\\dt'

docker-build:
	docker build -t $(IMAGE) .

up:
	docker compose up -d postgres

down:
	docker compose down
