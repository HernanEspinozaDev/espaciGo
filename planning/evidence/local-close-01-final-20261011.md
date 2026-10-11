# LOCAL-CLOSE-01 — conciliación y regresión integral final

Fecha: 2026-10-11 · conciliación posterior a PR #264; `main` en `6164ba0a929bd088e94a8914287f3bffe7899f94`. PR #262/#263 integran los slices #258/#259 y PR #264 integra V52. El código integrado de V52 coincide con el probado en `056205d`; los commits posteriores solo actualizaron documentación.

## Integración y estado operativo

- PR #263 está fusionado en `main`; #258 fue aceptada/Cerrada/Hecho en Projects por RQF-091/092 y #259 por RQF-177, únicamente por esos alcances. #246 permanece abierta hasta aceptación explícita de LOCAL-1.
- `bash scripts/dev-env.sh up -d` aplicó V50/V51 y, tras integrar #264, V52 incrementalmente. `schema_migrations` informa 52. PostgreSQL, Backend, Mailpit y mock quedaron saludables; `bash scripts/dev-env.sh verify-http` pasó liveness/readiness, CORS/preflight, HTML/config/JS/CSS del mock y conectividad mock→API. Los grants efectivos son `DELETE=false`, `UPDATE=false`, `EXECUTE` de la limpieza acotada `true`.
- `espacigo_pgdata` siguió presente. `.local/secrets` conservó propietario `nandev` y modo `0700`; no se imprimieron ni regeneraron secretos ni se borró volumen o datos.
- El runner retiró variables de conexión de desarrollo y usó PostgreSQL/PostGIS y Mailpit desechables por paquete. V52 se aplicó después al volumen persistente sin recrearlo.

### Comprobación posterior al merge #264

El diff entre el commit integral probado `056205d` y `main` `6164ba0` para migraciones, Backend, permisos, scripts y mock está vacío; los cambios intermedios son documentación. Por eso se reutiliza la regresión integral publicada y se ejecutaron los checks enfocados de V52:

```sh
GO_TEST_RUN='^TestPostgresPrivateSimulationSnapshotsRateAndDoesNotOccupyCalendar$' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/pricing
GO_TEST_RUN='^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces
bash scripts/dev-env.sh verify-http
```

Ambos tests PostgreSQL desechables pasaron. El primero comprueba el rol runtime real, el rechazo de `DELETE` directo y que no se puede limpiar tarifas antes del tombstone. El segundo comprueba retiro permitido cuando ya no quedan bloqueadores, preserva la reserva/snapshot histórico y ejercita el cleanup recuperable de archivos. `verify-http` confirmó salud, CORS/preflight y conectividad del mock. La consulta posterior al arranque indicó V52 y grants efectivos `f|f|t` (`tarifa_espacio DELETE`, `UPDATE`, función controlada `EXECUTE`).

## Corrección encontrada al ejecutar el gate final

La primera regresión encontró un test de login que esperaba entrega de aviso sincrónica y `ErrDelivery` dentro de la petición. El contrato integrado en V46 guarda la intención de bloqueo en el outbox transaccional; SMTP se ejecuta fuera del login. Se actualizó el test para exigir el resultado de bloqueo persistido, un único aviso durable y ausencia de envío directo. También se cambió el nombre del test obligatorio en `scripts/local-required-integrations.txt`. No se cambió el comportamiento productivo de login ni se amplió el outbox.

La prueba enfocada anterior al runner completo pasó:

```sh
GO_TEST_RUN='^TestAuthLoginSessionFailureRollsBackAndDurableBlockAlertRemainsVisible$' \
  bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity
```

## Regresión integral LOCAL-1 tras #258/#259

Comando ejecutado desde esta rama, con el manifiesto vigente:

```sh
bash scripts/test-local-qa.sh
```

La primera ejecución tras los merges falló en `pricing`: el rol runtime conservaba DELETE directo sobre `tarifa_espacio`, en conflicto con la inmutabilidad de tarifas. V52 reemplaza el grant por `purge_unreferenced_local_publication_rates(uuid)`, que solo permite eliminar tarifas sin cotizaciones después de retirar la publicación. La integración de permisos y retiro pasó; la segunda ejecución integral dio **PASS**.

El runner pasó sobre el código registrado en el commit probado `056205d` (`fix(db): scope deletion of retired publication rates`); la actualización de evidencia/matriz es documental y no altera el código ni el manifiesto.

- Mock: TypeScript compilado; **100/100** pruebas de estado, cero fallidas, omitidas o pendientes.
- Gate de manifiesto: **4/4** pruebas del validador pasaron para casos ausentes, omitidos y correctos.
- Go unitario: `go test -p 1 ./...` — PASS, con variables `TEST_DATABASE_URL`, SMTP y Mailpit de desarrollo retiradas del entorno.
- `go vet ./...` — PASS.
- PostgreSQL/PostGIS + Mailpit desechables: **83/83 integraciones obligatorias pasaron**; **0 faltantes, 0 omitidas, 0 SKIP y 0 fallidas**. Se registraron 121 eventos `pass` (incluye subtests). Paquetes: booking, gallery, identity, occupancy, pricing, reputation, spaces, verification, m02local y migrator. El rol runtime no tiene UPDATE/DELETE directo en tarifas; las simulaciones mantienen solo el DELETE que necesita la baja local.
- Estabilidad check-in: `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` **5/5 PASS** bajo PostgreSQL desechable.
- Higiene: `git diff --check` — PASS. El runner verificó la eliminación de contenedores, red, socket y directorio temporal desechables.

## Conciliación de LOCAL-1

La matriz completa por requisito, entidad, módulo, Issue y límite está en [`local-close-01-matrix-20261011.md`](local-close-01-matrix-20261011.md), reconciliada con `main` #264, DB02-01–09 y las 43 entidades. La matriz operativa también está en [`../matriz_cierre_backend_local.md`](../matriz_cierre_backend_local.md); plan/backlog reflejan aceptaciones, brechas y límites.

Además de los anteriores, #258 y #259 quedaron aceptadas por sus alcances. Las brechas locales requisito → código identificadas previamente (RQF-077, 091/092, 094/095/099/102, 177 y 198) ya tienen slices integrados y pruebas enfocadas. La primera corrida descubrió el grant DELETE directo excesivo sobre tarifas; V52 lo corrige. La segunda regresión integral pasó sin omisiones, el código probado coincide con el merge y V52 quedó aplicada incrementalmente al entorno persistente.

## Pendientes locales, dependencias y exclusiones

- **DB02-09:** RQF-213–218 tienen función local implementada en #183/#199/#251. El estado de Issues #31/#34 continúa OPEN; no se deriva cierre de los padres. El crosswalk de DB02-01–09 está en la matriz vigente.
- **Alcance local de padres aún no aceptado:** M01–M11 mantienen abiertas las Issues generales con criterios no aceptados. La tabla de padres distingue los cortes locales implementados de requisitos externos, diferidos y decisiones pendientes; no usar el estado OPEN como descripción de código faltante. En M11 se aceptan los tres reportes ratificados y las consultas de #250; los reportes adicionales no especificados están diferidos expresamente.
- **Permisos de tarifas integrados y comprobados:** V52 elimina DELETE directo al runtime y restringe la limpieza al retiro tombstone, manteniendo las tarifas referenciadas. El test de pricing, la prueba del flujo de eliminación y el runner integral pasaron en #264 (`056205d`); tras el merge se aplicó V52 incrementalmente y se verificaron los grants efectivos.
- **Privacidad/copia:** las bajas, plazos ratificados, purgador V47 y replay tras restauración tienen evidencia. Se conserva el volumen activo y el registro externo; las ubicaciones externas no comprobadas siguen desconocidas por decisión del propietario. No se borraron copias ni se declara supresión integral o anonimización.
- **Exclusiones/diferimientos ratificados:** proveedor PSP y sandbox real (#76/#78), firma jurídicamente válida, KYC/KYB documental/RUT/proveedores reales (#142), movimientos financieros reales, liquidación/comisión productiva y documento fiscal válido; promociones/campañas/NPS por MAP-07; realtime/avisos por mensaje; geocodificación de dirección privada; y GCP. Estas exclusiones no transforman fake local que siga pendiente en tarea externa.
- **Criterios que mantienen padres abiertos por exclusión productiva o decisión ratificada:** RQF-033 (cuenta bancaria/RUT real), RQF-039–044 y RQF-048–053 (verificación documental/proveedores reales, #142), RQF-080 (geocodificar dirección), RQF-131/133 (datos legales y proveedor de firma con validez jurídica), RQF-173 (transferencia bancaria real), RQF-176/211 (boleta electrónica fiscal), RQF-184 (búsqueda por RUT, excluida a favor de UUID sintético), RNF-017 (inmutabilidad productiva) y los reportes adicionales no especificados de RQF-183. RQF-183 local sí cubre los tres reportes expresamente ratificados. Las ubicaciones de respaldos fuera del inventario local autorizado siguen sin comprobarse; no se afirma supresión integral.
- **Issues padre:** #185/#40, #109/#110 y #111–#118 siguen abiertas por criterios generales. Mantener sus dependencias originales y no cerrarlas para aceptar las subentregas locales.

## Decisión de cierre

El runner integral pasó en `056205d`, y el código ejecutable coincide con el integrado en #264. Sin embargo, esta suite no verifica que el ZIP de titular contenga todos los dominios ya implementados. La revisión de `ownerexport.Repository.ExportOwnArchiveSections` y los lectores de `spaces`, `pricing`, `bookings`, `conversation` y `disputes` confirmó que no hay lector de `internal/contract`: faltan contrato propio, firmas/historial y el PDF privado propio disponible. Este es un **pendiente funcional local** que impide solicitar la aceptación de LOCAL-1. También queda pendiente ratificar si el almacenamiento/metadata especializado satisface la entidad común `documento` de MAP-02, además de las decisiones de retención para contratos/artefactos e historiales de publicación/eliminación; los datos actuales se conservan y no se asigna un plazo por analogía. #246 permanece abierta. Reutilizar la regresión actual hasta que cambie código; el runner no sustituye revisión de cobertura de exportación ni la aceptación del propietario.
