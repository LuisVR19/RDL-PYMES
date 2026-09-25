# syntax=docker/dockerfile:1
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gateway /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/gateway"]

# TODO(P7 · incremento 6): agregar el binario `replay` (reproductor de eventos para desarrollo) cuando existan
# las notificaciones en tiempo real.
