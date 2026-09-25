# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run build test test-isolation lint migrate-up migrate-status migrate-mark-baseline sqlc docker

run:
	$(GO) run ./cmd/api

build:
	$(GO) build ./...
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/migrate ./cmd/migrate

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# Suite de aislamiento entre tenants (incremento 10). Usa TEST_DATABASE_URL o el .env (login receivables_api).
test-isolation:
	$(GO) test -count=1 -tags=integration ./tests/isolation/...

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
