# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run build test test-integration test-isolation lint migrate-up migrate-status sqlc docker

run:
	$(GO) run ./cmd/api

build:
	$(GO) build ./...
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/migrate ./cmd/migrate

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# SQL de los adapters y tenancy contra la base dev con el login billing_api (TEST_DATABASE_URL o el .env).
# Lo que escriben lo revierten. Sin base, se salta con aviso.
test-integration:
	$(GO) test -count=1 -tags=integration ./internal/adapters/postgres/... ./tests/integration/...

# Suite de aislamiento entre tenants (criterio de terminado del planning). Usa TEST_DATABASE_URL o, si no está,
# el .env de la API (login billing_api). Sin base, se salta con aviso. Nunca borra datos.
test-isolation:
	$(GO) test -count=1 -tags=integration ./tests/isolation/...

lint:
	golangci-lint run --build-tags=integration ./...

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-status:
	$(GO) run ./cmd/migrate status

sqlc:
	sqlc generate

docker:
	# Contexto = carpeta padre: el build necesita ../RDL.Contracts (replace en go.mod).
	docker build -f Dockerfile -t billing-api:dev ..
