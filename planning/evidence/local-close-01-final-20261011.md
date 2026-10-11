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

El runner se ejecutó sobre el árbol que luego quedó registrado en el commit probado `5d635f826f6b1afeeb514148f5bc4c68c9b1550c`; esta actualización posterior del PR es documental y no altera el código ni el manifiesto probados.

- Mock: TypeScript compilado; **93/93** pruebas de estado, cero fallidas, omitidas o pendientes.
- Gate de manifiesto: **4/4** pruebas del validador pasaron para casos ausentes, omitidos y correctos.
- Go unitario: `go test -p 1 ./...` — PASS, con variables `TEST_DATABASE_URL`, SMTP y Mailpit de desarrollo retiradas del entorno.
- `go vet ./...` — PASS.
- PostgreSQL/PostGIS + Mailpit desechables: **83/83 integraciones obligatorias pasaron**; **0 faltantes, 0 omitidas, 0 SKIP y 0 fallidas**. Se registraron 121 eventos `pass` (incluye subtests). Paquetes integrados: booking, gallery, identity, occupancy, pricing, reputation, spaces, verification, m02local y migrator.
- Estabilidad check-in: `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` **5/5 PASS** bajo PostgreSQL desechable.
- Higiene: `git diff --check` — PASS. El runner verificó la eliminación de contenedores, red, socket y directorio temporal desechables.

## Conciliación de LOCAL-1

La matriz completa por requisito, entidad, módulo, Issue y límite está en [`local-close-01-matrix-20261011.md`](local-close-01-matrix-20261011.md), reconciliada con `main` hasta #254. La matriz operativa también está en [`../matriz_cierre_backend_local.md`](../matriz_cierre_backend_local.md); el plan y backlog se actualizaron para reflejar #200, #214, #222, #235, #247–#250, QA-01/02/03 y sus aceptaciones reales.

Los cortes locales #247–#250 están aceptados por sus alcances; #250 quedó cerrado/Hecho. La regresión encontró y corrigió una inconsistencia de test/manifiesto; no hubo integración obligatoria fallida u omitida. La revisión posterior de los requisitos originales sí encontró brechas funcionales locales fuera de ese manifiesto: **RQF-077, RQF-091/092, RQF-094/095/099/102, RQF-177 y RQF-198**. La descripción y dependencia de cada una está en la sección “Brechas funcionales locales obligatorias aún pendientes” de `local-close-01-matrix-20261011.md`. El runner PASS no demuestra esos criterios porque sus pruebas/funciones aún no existen.

## Pendientes locales, dependencias y exclusiones

- **Pendiente de decisión/trazabilidad:** DB02-09 conserva su estado previo. Las capacidades locales de preferencia, historial de claves y avisos están implementadas en cortes aceptados, pero no se declara resuelta toda la decisión general ni se cierra la Issue histórica por asociación.
- **Alcance local de padres aún no aceptado:** M04/M05/M06 y M11 tienen flujos locales útiles integrados, pero sus Issues generales permanecen abiertas y no todos los criterios de marketplace, administración y políticas generales tienen aceptación individual. En M11 se aceptan los tres reportes ratificados y las consultas de #250; los reportes adicionales no especificados están diferidos expresamente.
- **Brechas locales obligatorias:** RQF-077 (selección de portada); RQF-091/092 (eliminación de publicación con bloqueo por reserva futura); RQF-094 (búsqueda textual), RQF-095 (mapa interactivo), RQF-099 (filtro de superficie mínima), RQF-102 (limpiar filtros), RQF-177 (pasar la reserva a `cerrada` tras liquidación confirmada) y RQF-198 (editar modalidad tarifaria). #52–#61, #62–#68 y #109/#110 no pueden declararse completos hasta resolver los IDs aplicables. RQF-091/092 también requieren fijar cómo conservar los vínculos históricos sujetos a retención.
- **Privacidad/copia:** las bajas, plazos ratificados, purgador V47 y replay tras restauración tienen evidencia. Se conserva el volumen activo y el registro externo; las ubicaciones externas no comprobadas siguen desconocidas por decisión del propietario. No se borraron copias ni se declara supresión integral o anonimización.
- **Exclusiones/diferimientos ratificados:** proveedor PSP y sandbox real (#76/#78), firma jurídicamente válida, KYC/KYB documental/RUT/proveedores reales (#142), movimientos financieros reales, liquidación/comisión productiva y documento fiscal válido; promociones/campañas/NPS por MAP-07; realtime/avisos por mensaje; geocodificación de dirección privada; y GCP. Estas exclusiones no transforman fake local que siga pendiente en tarea externa.
- **Criterios que mantienen padres abiertos por exclusión productiva o decisión ratificada:** RQF-033 (cuenta bancaria/RUT real), RQF-039–044 y RQF-048–053 (verificación documental/proveedores reales, #142), RQF-080 (geocodificar dirección), RQF-131/133 (datos legales y proveedor de firma con validez jurídica), RQF-173 (transferencia bancaria real), RQF-176/211 (boleta electrónica fiscal), RQF-184 (búsqueda por RUT, excluida a favor de UUID sintético), RNF-017 (inmutabilidad productiva) y los reportes adicionales no especificados de RQF-183. RQF-183 local sí cubre los tres reportes expresamente ratificados. Las ubicaciones de respaldos fuera del inventario local autorizado siguen sin comprobarse; no se afirma supresión integral.
- **Issues padre:** #185/#40, #109/#110 y #111–#118 siguen abiertas por criterios generales. Mantener sus dependencias originales y no cerrarlas para aceptar las subentregas locales.

## Decisión de cierre

El runner queda reproducible y aprobado técnicamente, pero LOCAL-1 aún tiene las brechas funcionales locales listadas. #246 permanece abierta y en curso; ni el runner, ni la aceptación de #250 ni los cortes aceptados son por sí solos la aceptación final del propietario.
