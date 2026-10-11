# LOCAL-CLOSE-01 — conciliación y regresión integral final

Fecha: 2026-10-11 · rama de cierre: `codex/local-close-01-final` · `main` comprobado en PR #254, merge `1b57df27f6991833cc73f3bdf7479a5ecea2c704`.

## Integración y estado operativo

- PR #254 está fusionado en `main`. #250 fue aceptada por su alcance, cerrada y marcada **Hecho** en Projects después de `scripts/dev-env.sh verify-http`; #246 permanece abierta y **En curso** hasta la aceptación final del propietario.
- `bash scripts/dev-env.sh verify-http` pasó después del merge y otra vez después de la regresión: liveness/readiness de API/PostgreSQL, CORS y preflight, HTML/config/JS/CSS del mock y conectividad mock→API.
- Backend, base de datos, Mailpit y mock estaban saludables. `espacigo_pgdata` continuó presente y montado; `.local/secrets` conservó propietario `nandev` y modo `0700`. No se imprimieron ni regeneraron secretos ni se modificaron intencionalmente datos de desarrollo.
- PR #254 no añade migración; no se ejecutó migración sobre el volumen persistente. El runner quitó las variables de conexión de desarrollo y usó instancias PostgreSQL/PostGIS y Mailpit desechables por paquete.

## Corrección encontrada al ejecutar el gate final

La primera regresión encontró un test de login que esperaba entrega de aviso sincrónica y `ErrDelivery` dentro de la petición. El contrato integrado en V46 guarda la intención de bloqueo en el outbox transaccional; SMTP se ejecuta fuera del login. Se actualizó el test para exigir el resultado de bloqueo persistido, un único aviso durable y ausencia de envío directo. También se cambió el nombre del test obligatorio en `scripts/local-required-integrations.txt`. No se cambió el comportamiento productivo de login ni se amplió el outbox.

La prueba enfocada anterior al runner completo pasó:

```sh
GO_TEST_RUN='^TestAuthLoginSessionFailureRollsBackAndDurableBlockAlertRemainsVisible$' \
  bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity
```

## Regresión integral final

Comando ejecutado desde esta rama, con el manifiesto vigente:

```sh
bash scripts/test-local-qa.sh
```

Resultado final: **PASS**.

- Mock: TypeScript compilado; **93/93** pruebas de estado, cero fallidas, omitidas o pendientes.
- Gate de manifiesto: **4/4** pruebas del validador pasaron para casos ausentes, omitidos y correctos.
- Go unitario: `go test -p 1 ./...` — PASS, con variables `TEST_DATABASE_URL`, SMTP y Mailpit de desarrollo retiradas del entorno.
- `go vet ./...` — PASS.
- PostgreSQL/PostGIS + Mailpit desechables: **83/83 integraciones obligatorias pasaron**; **0 faltantes, 0 omitidas, 0 SKIP y 0 fallidas**. Se registraron 121 eventos `pass` (incluye subtests). Paquetes integrados: booking, gallery, identity, occupancy, pricing, reputation, spaces, verification, m02local y migrator.
- Estabilidad check-in: `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` **5/5 PASS** bajo PostgreSQL desechable.
- Higiene: `git diff --check` — PASS. El runner verificó la eliminación de contenedores, red, socket y directorio temporal desechables.

## Conciliación de LOCAL-1

La matriz completa por requisito, entidad, módulo, Issue y límite está en [`local-close-01-matrix-20261011.md`](local-close-01-matrix-20261011.md), reconciliada con `main` hasta #254. La matriz operativa también está en [`../matriz_cierre_backend_local.md`](../matriz_cierre_backend_local.md); el plan y backlog se actualizaron para reflejar #200, #214, #222, #235, #247–#250, QA-01/02/03 y sus aceptaciones reales.

Los cortes locales #247–#250 están aceptados por sus alcances; #250 quedó cerrado/Hecho. La regresión encontró y corrigió la única inconsistencia de test/manifiesto. No se encontró una integración obligatoria fallida u omitida ni otra brecha funcional dentro de esos cortes autorizados. Esto **no cierra ni completa automáticamente** los padres generales: #31/#34, #37–#43, #45/#48–#50, #52–#79, #80–#110, #111–#118 y #185/#40 conservan los criterios de sus Issues.

## Pendientes locales, dependencias y exclusiones

- **Pendiente de decisión/trazabilidad:** DB02-09 conserva su estado previo. Las capacidades locales de preferencia, historial de claves y avisos están implementadas en cortes aceptados, pero no se declara resuelta toda la decisión general ni se cierra la Issue histórica por asociación.
- **Alcance local de padres aún no aceptado:** M04/M05/M06 y M11 tienen flujos locales útiles integrados, pero sus Issues generales permanecen abiertas y no todos los criterios de marketplace, administración y políticas generales tienen aceptación individual. En M11 se aceptan los tres reportes ratificados y las consultas de #250; los reportes adicionales no especificados están diferidos expresamente.
- **Privacidad/copia:** las bajas, plazos ratificados, purgador V47 y replay tras restauración tienen evidencia. Se conserva el volumen activo y el registro externo; las ubicaciones externas no comprobadas siguen desconocidas por decisión del propietario. No se borraron copias ni se declara supresión integral o anonimización.
- **Exclusiones/diferimientos ratificados:** proveedor PSP y sandbox real (#76/#78), firma jurídicamente válida, KYC/KYB documental/RUT/proveedores reales (#142), movimientos financieros reales, liquidación/comisión productiva y documento fiscal válido; promociones/campañas/NPS por MAP-07; realtime/avisos por mensaje; geocodificación de dirección privada; y GCP. Estas exclusiones no transforman fake local que siga pendiente en tarea externa.
- **Issues padre:** #185/#40, #109/#110 y #111–#118 siguen abiertas por criterios generales. Mantener sus dependencias originales y no cerrarlas para aceptar las subentregas locales.

## Decisión de cierre

El resultado reproducible de LOCAL-1 queda presentado para aceptación humana. #246 permanece abierta y en curso; el runner exitoso, la aceptación de #250 y los cortes aceptados no son por sí solos la aceptación final del propietario.
