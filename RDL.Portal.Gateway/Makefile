# Cargar variables antes de usar make (make no entiende las comillas del .env):
#   set -a; . ./.env; set +a

GO ?= go

.PHONY: run build test test-isolation e2e lint docker routes

run:
	$(GO) run ./cmd/gateway

build:
	$(GO) build ./...
	$(GO) build -o bin/gateway ./cmd/gateway

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# Aislamiento entre organizaciones sobre el router real, con tokens reales de dos usuarios de Supabase dev y las
# APIs corriendo en local. Necesita Platform arriba y .e2e.local (E2E_EMAIL/…2); si falta algo, se salta con aviso.
test-isolation:
	$(GO) test -tags=integration -count=1 -v ./tests/isolation/...

# Punta a punta contra el gateway ya corriendo (make run en otra terminal).
e2e:
	bash scripts/dev/e2e.sh

lint:
	golangci-lint run --build-tags=integration ./...

# Imprime la tabla de rutas tal como la expone el gateway. Útil para revisar un PR que agrega pantallas.
routes:
	$(GO) run ./cmd/gateway -routes

docker:
	docker build -t portal-gateway:dev .

# TODO(P7 · incremento 6): `replay` (adapter de desarrollo que reproduce los eventos de examples/events/
# del repo de contratos) y, en `test-isolation`, dos suscriptores de organizaciones distintas cuando existan
# las notificaciones en tiempo real.
