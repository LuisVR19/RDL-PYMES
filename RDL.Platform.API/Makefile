# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run build test test-isolation lint migrate-up migrate-status sqlc docker

run:
	$(GO) run ./cmd/api

build:
	$(GO) build ./...
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/migrate ./cmd/migrate

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# Suite de aislamiento entre tenants (criterio de terminado del planning). Usa TEST_DATABASE_URL o, si no está,
# el .env de la API (login platform_api). Sin base, se salta con aviso. Nunca borra datos.
test-isolation:
	$(GO) test -count=1 -tags=integration ./tests/isolation/...

lint:
	golangci-lint run ./...

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-status:
	$(GO) run ./cmd/migrate status

sqlc:
	sqlc generate

docker:
	docker build -t platform-api:dev .
