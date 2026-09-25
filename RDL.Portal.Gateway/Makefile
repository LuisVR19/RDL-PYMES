# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run build test test-race lint docker routes

run:
	$(GO) run ./cmd/gateway

build:
	$(GO) build ./...
	$(GO) build -o bin/gateway ./cmd/gateway

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

lint:
	golangci-lint run ./...

# Imprime la tabla de rutas tal como la expone el gateway. Útil para revisar un PR que agrega pantallas.
routes:
	$(GO) run ./cmd/gateway -routes

docker:
	docker build -t portal-gateway:dev .

# TODO(P7 · incremento 6): `replay` (adapter de desarrollo que reproduce los eventos de examples/events/
# del repo de contratos) y `test-isolation` (dos suscriptores de organizaciones distintas) cuando existan
# las notificaciones en tiempo real.
