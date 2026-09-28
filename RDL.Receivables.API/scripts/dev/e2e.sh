#!/usr/bin/env sh
# E2E de Receivables contra dev (solo dev, nunca producción). Necesita el .env de la raíz con el login
# receivables_api e ISOLATION_ORG_A/B.
#
# 1. reproduce InvoiceIssued con los ejemplos de contratos (cmd/replay), dos veces: la segunda son duplicados;
# 2. corre tests/e2e: pago que cancela la factura, ReceivableSettled en el outbox, anulación del pago con el saldo
#    de vuelta, notas de crédito y débito, anulación de factura, concurrencia, aging y cobranza.
set -eu
cd "$(dirname "$0")/../.."

echo "== replay de InvoiceIssued (ejemplos de contratos)"
go run ./cmd/replay -file ../RDL.Contracts/examples/events/invoice-issued.v1.json
echo "== replay otra vez: no debe duplicar nada"
go run ./cmd/replay -file ../RDL.Contracts/examples/events/invoice-issued.v1.json 2>&1 | grep -c '"outcome":"duplicate"'

echo "== tests/e2e"
go test -count=1 -tags=integration ./tests/e2e/...
