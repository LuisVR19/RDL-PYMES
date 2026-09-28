# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run consumer run-consumer replay build test test-integration test-isolation test-e2e lint migrate-up migrate-status migrate-mark-baseline sqlc docker

run:
	$(GO) run ./cmd/api

# Consumidor de eventos. Hasta P2 no tiene transporte (TODO(P2)): los eventos entran con make replay.
consumer run-consumer:
	$(GO) run ./cmd/consumer

# Reproduce eventos desde un archivo, por el mismo procesamiento que el consumidor. Solo dev y local.
#   make replay FILE=../RDL.Contracts/examples/events/invoice-issued.v1.json [SET=invalid]
replay:
	$(GO) run ./cmd/replay -file $(FILE) -set $(or $(SET),valid)

build:
	$(GO) build ./...
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/consumer ./cmd/consumer
	$(GO) build -o bin/replay ./cmd/replay
	$(GO) build -o bin/migrate ./cmd/migrate

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# Integración contra dev con el login receivables_api. Cada test hace rollback: no deja datos.
test-integration:
	$(GO) test -count=1 -tags=integration ./tests/integration/...

# Suite de aislamiento entre tenants (6 criterios, ADR 0008). Usa TEST_DATABASE_URL o el .env (login receivables_api)
# e ISOLATION_ORG_A/B (dos organizaciones de prueba de dev). Deja cuentas y pagos de prueba en esas organizaciones.
test-isolation:
	$(GO) test -count=1 -tags=integration ./tests/isolation/...

# De punta a punta contra dev: factura, pago, ReceivableSettled, anulación, notas, aging y cobranza.
test-e2e:
	$(GO) test -count=1 -tags=integration ./tests/e2e/...

lint:
	golangci-lint run --build-tags=integration ./...

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-status:
	$(GO) run ./cmd/migrate status

# Solo una vez en dev: registra la baseline 00001 como aplicada sin ejecutarla (el schema lo creó database-platform).
migrate-mark-baseline:
	$(GO) run ./cmd/migrate mark-baseline

# El contexto es el monorepo: a partir del incremento 3 el build necesita ../RDL.Contracts (replace en go.mod).
docker:
	docker build -f Dockerfile -t receivables-api:dev ..

# Regenera internal/adapters/postgres/db desde queries/ (sqlc v1.31.1, igual que Platform).
sqlc:
	sqlc generate
