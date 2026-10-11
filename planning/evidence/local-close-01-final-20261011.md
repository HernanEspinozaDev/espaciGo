# LOCAL-CLOSE-01 — conciliación y regresión integral final

Fecha: 2026-10-11 · conciliación posterior a PR #263; `main` en `a0a3ad6130745d3d2126eb4c1c92805eacc39084`. PR #262/#263 integran los slices #258/#259. V52 está en PR #264 (`codex/local-close-rate-delete-privilege`), código probado `056205d`, pendiente de revisión/merge.

## Integración y estado operativo

- PR #263 está fusionado en `main`; #258 fue aceptada/Cerrada/Hecho en Projects por RQF-091/092 y #259 por RQF-177, únicamente por esos alcances. #246 permanece abierta hasta aceptación explícita de LOCAL-1.
- `bash scripts/dev-env.sh up -d` aplicó V50/V51 incrementalmente. PostgreSQL, Backend, Mailpit y mock quedaron saludables; `bash scripts/dev-env.sh verify-http` pasó liveness/readiness, CORS/preflight, HTML/config/JS/CSS del mock y conectividad mock→API.
- `espacigo_pgdata` siguió presente. `.local/secrets` conservó propietario `nandev` y modo `0700`; no se imprimieron ni regeneraron secretos ni se borró volumen o datos.
- El runner retiró variables de conexión de desarrollo y usó PostgreSQL/PostGIS y Mailpit desechables por paquete. V52 aún no se aplicó al volumen persistente.

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

La matriz completa por requisito, entidad, módulo, Issue y límite está en [`local-close-01-matrix-20261011.md`](local-close-01-matrix-20261011.md), reconciliada con `main` hasta #263 y anotada con V52/PR #264 pendiente de integración. La matriz operativa también está en [`../matriz_cierre_backend_local.md`](../matriz_cierre_backend_local.md); plan/backlog reflejan las aceptaciones locales y sus límites.

Además de los anteriores, #258 y #259 quedaron aceptadas por sus alcances. Las brechas locales requisito → código identificadas previamente (RQF-077, 091/092, 094/095/099/102, 177 y 198) ya tienen slices integrados y pruebas enfocadas. La primera corrida descubrió el grant DELETE directo excesivo sobre tarifas; V52 lo corrige y la segunda corrida pasó sin omisiones. V52 requiere revisión/merge y aplicación incremental para integrarse también al entorno persistente.

## Pendientes locales, dependencias y exclusiones

- **Pendiente de decisión/trazabilidad:** DB02-09 conserva su estado previo. Las capacidades locales de preferencia, historial de claves y avisos están implementadas en cortes aceptados, pero no se declara resuelta toda la decisión general ni se cierra la Issue histórica por asociación.
- **Alcance local de padres aún no aceptado:** M04/M05/M06 y M11 tienen flujos locales útiles integrados, pero sus Issues generales permanecen abiertas y no todos los criterios de marketplace, administración y políticas generales tienen aceptación individual. En M11 se aceptan los tres reportes ratificados y las consultas de #250; los reportes adicionales no especificados están diferidos expresamente.
- **Permisos de tarifas pendiente de integración:** V52 elimina DELETE directo al runtime y restringe la limpieza de tarifas al retiro tombstone sin cotizaciones. El test de pricing, la prueba del flujo de eliminación y el runner integral pasaron en PR #264 (`codex/local-close-rate-delete-privilege`, `056205d`); no aplicar V52 al volumen hasta revisar/fusionar su PR.
- **Privacidad/copia:** las bajas, plazos ratificados, purgador V47 y replay tras restauración tienen evidencia. Se conserva el volumen activo y el registro externo; las ubicaciones externas no comprobadas siguen desconocidas por decisión del propietario. No se borraron copias ni se declara supresión integral o anonimización.
- **Exclusiones/diferimientos ratificados:** proveedor PSP y sandbox real (#76/#78), firma jurídicamente válida, KYC/KYB documental/RUT/proveedores reales (#142), movimientos financieros reales, liquidación/comisión productiva y documento fiscal válido; promociones/campañas/NPS por MAP-07; realtime/avisos por mensaje; geocodificación de dirección privada; y GCP. Estas exclusiones no transforman fake local que siga pendiente en tarea externa.
- **Criterios que mantienen padres abiertos por exclusión productiva o decisión ratificada:** RQF-033 (cuenta bancaria/RUT real), RQF-039–044 y RQF-048–053 (verificación documental/proveedores reales, #142), RQF-080 (geocodificar dirección), RQF-131/133 (datos legales y proveedor de firma con validez jurídica), RQF-173 (transferencia bancaria real), RQF-176/211 (boleta electrónica fiscal), RQF-184 (búsqueda por RUT, excluida a favor de UUID sintético), RNF-017 (inmutabilidad productiva) y los reportes adicionales no especificados de RQF-183. RQF-183 local sí cubre los tres reportes expresamente ratificados. Las ubicaciones de respaldos fuera del inventario local autorizado siguen sin comprobarse; no se afirma supresión integral.
- **Issues padre:** #185/#40, #109/#110 y #111–#118 siguen abiertas por criterios generales. Mantener sus dependencias originales y no cerrarlas para aceptar las subentregas locales.

## Decisión de cierre

El runner integral pasó en el seguimiento con V52. No queda otra brecha funcional local obligatoria conocida en los RQF auditados; resta integrar V52 y aplicarla incrementalmente al entorno. #246 permanece abierta y en curso. Ni el runner ni las aceptaciones de slices sustituyen la aceptación final del propietario.
