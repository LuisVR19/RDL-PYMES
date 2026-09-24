GO ?= go
CLI := $(GO) run ./cmd/contractsctl
# Directorio con la versión publicada contra la que compara breaking (por ejemplo el último tag descomprimido).
BASE ?=

.PHONY: build test lint validate contracts-lint breaking check release

build:
	$(GO) build ./...
	$(GO) build -o bin/contractsctl ./cmd/contractsctl

test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

lint:
	golangci-lint run ./...

validate:
	$(CLI) validate

contracts-lint:
	$(CLI) lint

# Uso: make breaking BASE=../contracts-v0.1.0
breaking:
	@test -n "$(BASE)" || (echo "falta BASE=<directorio de la versión publicada>" && exit 1)
	$(CLI) breaking -base $(BASE)

# Todo lo que corre el CI.
check: build test lint validate contracts-lint

# Uso: make release VERSION=v0.2.0 (requiere CHANGELOG actualizado y check en verde).
release: check
	@test -n "$(VERSION)" || (echo "falta VERSION=vX.Y.Z" && exit 1)
	git tag -a $(VERSION) -m "contracts $(VERSION)"
	@echo "Tag $(VERSION) creado. Publicar con: git push origin $(VERSION)"
