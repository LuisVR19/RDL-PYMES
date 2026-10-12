# syntax=docker/dockerfile:1
# Contexto de build: el monorepo (make docker). Hasta que exista el tag del repo de contratos, go.mod lo toma de
# ../RDL.Contracts con un replace, así que la imagen copia los dos directorios.
# TODO(P5, incremento 7): la validación XSD usará libxml2 vía cgo (docs/PLAN.md §3); entonces la imagen final pasa a
# debian-slim con libxml2 y deja de ser distroless estática.
FROM golang:1.27 AS build
WORKDIR /src
COPY RDL.Contracts/ RDL.Contracts/
COPY RDL.EInvoice.API/go.mod RDL.EInvoice.API/go.sum RDL.EInvoice.API/
WORKDIR /src/RDL.EInvoice.API
RUN go mod download
COPY RDL.EInvoice.API/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
USER nonroot:nonroot
# La misma imagen sirve para la API (por defecto) y el consumidor: ENTRYPOINT ["/usr/local/bin/consumer"], puerto 8085.
# El worker (8086) llega con el incremento 9.
EXPOSE 8082 8085
ENTRYPOINT ["/usr/local/bin/api"]
