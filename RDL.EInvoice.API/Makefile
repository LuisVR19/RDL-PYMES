# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run consumer run-consumer replay import-catalogs build test test-integration test-isolation lint migrate-up migrate-up-by-one migrate-status migrate-mark-baseline sqlc docker

# run-worker y test-sandbox llegan con el incremento 9 (docs/PLAN.md §4).

run:
	$(GO) run ./cmd/api

# Consumidor de eventos (:8085). Hasta P2 no tiene transporte (TODO(P2)): los eventos entran con make replay.
consumer run-consumer:
	$(GO) run ./cmd/consumer

# Reproduce eventos desde un archivo, por el mismo procesamiento que el consumidor. Solo dev y local.
#   make replay FILE=../RDL.Contracts/examples/events/invoice-issued.v1.json [SET=invalid]
replay:
	$(GO) run ./cmd/replay -file $(FILE) -set $(or $(SET),valid)

# Carga los catálogos de catalogs/ (transcritos de docs/hacienda/) con el login fiscal_migrate. Idempotente.
import-catalogs:
	$(GO) run ./cmd/import-catalogs

build:
	$(GO) build ./...
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/consumer ./cmd/consumer
	$(GO) build -o bin/replay ./cmd/replay
	$(GO) build -o bin/import-catalogs ./cmd/import-catalogs
	$(GO) build -o bin/migrate ./cmd/migrate

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# Integración contra dev (login fiscal_api): consumidor, inbox, dead letter y rutas internas. Deja documentos de
# prueba en ISOLATION_ORG_A y un perfil fiscal ficticio si no tenía.
test-integration:
	$(GO) test -count=1 -tags=integration ./tests/integration/...

# Suite de aislamiento entre tenants (crece hasta los 6 criterios en el incremento 13).
test-isolation:
	$(GO) test -count=1 -tags=integration ./tests/isolation/...

lint:
	golangci-lint run --build-tags=integration ./...

migrate-up:
	$(GO) run ./cmd/migrate up

# Expand → migrate → contract: aplica solo la siguiente migración pendiente.
migrate-up-by-one:
	$(GO) run ./cmd/migrate up-by-one

migrate-status:
	$(GO) run ./cmd/migrate status

# Solo una vez en dev: registra la baseline 00001 como aplicada sin ejecutarla (el schema lo creó database-platform).
migrate-mark-baseline:
	$(GO) run ./cmd/migrate mark-baseline

# Regenera internal/adapters/postgres/db desde queries/ (sqlc v1.31.1, igual que Platform).
sqlc:
	sqlc generate

# El contexto es el monorepo: go.mod toma el repo de contratos de ../RDL.Contracts (replace).
docker:
	docker build -f Dockerfile -t einvoice-api:dev ..
