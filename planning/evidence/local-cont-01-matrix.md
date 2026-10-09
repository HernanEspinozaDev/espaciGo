# LOCAL-CONT-01 — matriz de implementación y evidencia

Estado: listo para revisión en PR. Issue local #216, hija de #80; los padres #80–#87 siguen abiertos.

| Criterio | Resultado del corte | Evidencia nueva |
|---|---|---|
| Contrato inicia desde reserva aprobada, snapshot conserva intervalo, monto, moneda, zona, condiciones y política | Implementado localmente; el recuperador idempotente termina creaciones interrumpidas después de aprobación | `V000034`, `internal/adapters/postgres/contracts/repository.go`, `EnsureApproved` |
| Solo anfitrión/arrendatario leen; terceros reciben 404 | Implementado en repositorio y handler | Prueba PostgreSQL de acceso de tercero añadida al lifecycle M06 |
| Firma parcial y final por los dos participantes; repetición sin historial duplicado | Implementado con bloqueo común de reserva | Pruebas PostgreSQL de ambas firmas y replay |
| Rechazo terminal del firmante, sin cancelación inmediata | Implementado; M06 queda `firma_parcial` hasta el límite | Prueba PostgreSQL de rechazo y consulta posterior |
| Inicio estricto anterior para las firmas; vencimiento `>= start_at` | Implementado con reloj consultado después del lock | Prueba en límite exacto y carreras concurrentes signo/worker |
| Cancelación y liberación de ocupación atómicas; devolución única por importe fake confirmado | Implementado en M06; estado `cancelada_por_firma`; la devolución queda como obligación fake reutilizando el endpoint existente | Pruebas PostgreSQL de una transición, una devolución e importe total |
| Artefacto privado | PDF sintético, metadata/bytes bajo owner M09, cifrado AES-256-GCM; descarga autenticada solo tras ambas firmas | Pruebas de magic/EOF, cifrado, descifrado y detección de manipulación; prueba PostgreSQL valida PDF descifrado |
| Mock y sesión | Controles de participante, rechazo, firma, estado, historial y descarga; invalida operaciones al cambiar reserva/sesión | `npm --prefix mock run test:contract-actions`; compilación TS |
| OpenAPI | Rutas de creación/consulta/firma/rechazo/descarga y esquemas añadidos | `planning/openapi.yaml`; parse YAML correcto y 5 rutas de contrato |

## Límite de aceptación

El corte es exclusivamente sintético y cada interfaz/artefacto lo identifica como ensayo sin validez jurídica. No prueba un proveedor de firma, callback externo ni validez legal. El recuperador local busca contratos faltantes tras aprobación y cancela reservas aprobadas sin contrato/firma cuando el reloj Backend llega al inicio. Los avisos durables de contrato, exportación del nuevo artefacto por participante y retención productiva quedan fuera y no cierran M07 ni privacidad.

## Comprobaciones

Validación completada en esta rama:

- `bash scripts/test-m06-local-booking-postgres.sh` — PASS sobre PostgreSQL/PostGIS desechable. Incluye rol runtime, ciclo de reserva/pago fake, dos participantes y tercero, aprobación/rechazo, firma parcial/completa, documento PDF descifrado, límite temporal exacto y carreras de firma con worker y bloqueo. Una expiración cancela/libera y crea exactamente una obligación por el 100 % confirmado; reintentar worker/firma no duplica efectos.
- `go test ./internal/contract/... ./internal/adapters/postgres/contracts ./internal/adapters/postgres/documents ./internal/booking/transport/http ./cmd/api ./internal/dbbootstrap` — PASS.
- `go vet ./internal/contract/... ./internal/adapters/postgres/contracts ./internal/adapters/postgres/documents ./cmd/api` — PASS.
- `npm --prefix mock run test:contract-actions` — PASS, 3 pruebas del recorrido/guardas de sesión.
- Compilación TypeScript del mock y parseo de `planning/openapi.yaml` — PASS.
- `git diff --check` — PASS.

No se aplicaron migraciones a la base persistente ni se reiniciaron servicios o secretos. No se ejecutó un recorrido de navegador contra la base de desarrollo; la verificación end-to-end del corte está en la integración PostgreSQL desechable y las pruebas enfocadas del mock. No se ejecutaron suites ajenas a este corte ni pruebas/despliegues de GCP.
