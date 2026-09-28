# syntax=docker/dockerfile:1
# Contexto de build: el monorepo (make docker). Hasta que exista el tag del repo de contratos, go.mod lo toma de
# ../RDL.Contracts con un replace, así que la imagen copia los dos directorios.
FROM golang:1.27 AS build
WORKDIR /src
COPY RDL.Contracts/ RDL.Contracts/
COPY RDL.Receivables.API/go.mod RDL.Receivables.API/go.sum RDL.Receivables.API/
WORKDIR /src/RDL.Receivables.API
RUN go mod download
COPY RDL.Receivables.API/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/consumer ./cmd/consumer \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/replay ./cmd/replay \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
USER nonroot:nonroot
# La misma imagen sirve para la API (por defecto) y el consumidor: ENTRYPOINT ["/usr/local/bin/consumer"], puerto 8084.
EXPOSE 8083 8084
ENTRYPOINT ["/usr/local/bin/api"]
