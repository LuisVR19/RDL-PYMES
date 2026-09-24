# syntax=docker/dockerfile:1
# Contexto de build: la carpeta que contiene RDL.Billing.API y RDL.Contracts (go.mod usa replace ../RDL.Contracts
# mientras el módulo de contratos no tenga versión publicada). Ver `make docker`.
FROM golang:1.27 AS build
WORKDIR /src
COPY RDL.Contracts/ ./RDL.Contracts/
COPY RDL.Billing.API/go.mod RDL.Billing.API/go.sum ./RDL.Billing.API/
WORKDIR /src/RDL.Billing.API
RUN go mod download
COPY RDL.Billing.API/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/migrate /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/api"]
