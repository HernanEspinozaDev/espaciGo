# DEMO-01 — fuente compartida y composición Go local (parcial)

- Baseline del candidato: `f3671834727417733a11132aef783fa9941ba012`, worktree `espaciGo-rental-demo-reconciled`.
- Alcance: actor/estado sintético en memoria, reiniciable, sin PostgreSQL, proveedores, KYC, contrato ni pago reales. No se modifica el montaje de `cmd/api`.
- Contratos añadidos bajo `/local/rental-demo` a OpenAPI reconciliado; cliente/inventario/allowlist regenerados y `api-check` PASS. El gateway de producto rechaza ese prefijo antes de forwardear una sesión real (RED HTTP200, GREEN404 con cero fetch).

## Implementación y revisión

`internal/rentaldemo` contiene calendario de mes con fechas civiles y slots, configuración/recurrencia sintética, bloqueos propios con revisión, cotización sin retención, confirmación atómica bajo mutex con ocupación solo en memoria, intención idempotente y lectura de resultado por actor. `cmd/rentaldemo` arranca únicamente esta composición y rechaza listeners no numéricos loopback; no importa arranque de API, DB ni workers.

La revisión del padre no aceptó las entregas parciales como terminadas. Reprodujo y corrigió:

- Confirmación wire con snapshot vacío e ID ausente por campos sombra: RED por campos faltantes/vacíos; GREEN con snapshot completo serializado.
- Cotización expirada exactamente en now aceptada, y confirmación después del inicio: RED reprodujo ocupación incorrecta; GREEN rechaza antes de ocupar.
- Fin diario al cruzar mes/año bisiesto sin candidato: RED en enero31 y febrero29; GREEN opera fechas civiles y resuelve primer instante local.
- Aniversario mensual después de medianoche omitida conservaba 01:00 indebidamente: RED septiembre6→octubre6 en Santiago, GREEN usa primer instante de fecha objetivo.
- Día horario parcial presentado como totalmente disponible: RED status available, GREEN partial.
- Mes público retenía slice Blocks vacío en memoria: corregido a ausencia pública, conservando array vacío requerido para owner mediante DTO del handler.

## Pruebas ejecutadas por el padre

- `go test -race ./internal/rentaldemo ./cmd/rentaldemo -count=1` PASS.
- `go vet ./internal/rentaldemo ./cmd/rentaldemo` PASS.
- `TestOneHTTPSourceCoordinatesOwnerPublicQuoteAndPaidDemo` recorre un único handler/store real: mes público/owner→bloqueo→readback público→retiro→cotización sin hold→confirmación `paid_simulated`→ambos meses sin slot→replay misma intención sin duplicar→GET intento.
- Concurrencia del store: dos cotizaciones del mismo intervalo, exactamente una confirmación ganadora. No prueba SQL ni pagos reales.
- Pruebas civiles cubren día23h/25h Santiago, bisiesto, cruce de mes, mes clamped y hora repetida New York. Fuera de fuente sintética no se infieren reglas adicionales.
- Pruebas de handler: sesiones sintéticas separadas, revocación/vencimiento, actor renter sin permisos owner, query/JSON duplicados y campos desconocidos rechazados, DTO público sin blocks/revision, host/origin remotos denegados.
- `git diff --check` PASS.

## Ejecución real local

Se construyó binario en scratch y arrancó proceso propio `proc_d1d2000bf4f4`, PID649646, listener `127.0.0.1:18117`, reloj sintético fijo `2030-01-15T12:00:00Z`, entorno allowlist sin variables DB/proveedor. Preflight confirmó puerto libre.

Readback HTTP real por urllib (sin stubs): 3 espacios sintéticos, mes de31 días, 23 intervalos horarios consultables para el día16, mismos días en owner y público, array blocks vacío solo owner, login/logout sintético PASS. No se imprimió ni guardó cookie/token. Solo se consultó calendario y sesión; no se crearon bloqueos ni ocupaciones en este preflight.

## Límites y siguiente gate

Gateway BFF y consumidores frontend en integración; una incompatibilidad concreta con cookies Go Max-Age/logout204 se envió a corrección. No se afirma todavía navegador→BFF→Go, accesibilidad, capturas ni cierre DEMO-01–05. El proceso se mantiene temporalmente como recurso propio para ese ensayo, sin tocar otros listeners ni demos retenidas. Al terminar debe retirarse únicamente este proceso. La inspección independiente source-only está en curso.
