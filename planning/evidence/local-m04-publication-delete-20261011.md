# LOCAL-M04-PUBLICATION-DELETE-01 — evidencia de la entrega

Fecha: 2026-10-11. PR #262 está fusionado (`4f96f81fe11093fc56c3755b9d35fbc29ac94d8f`) y #258 fue aceptada/Cerrada/Hecho por RQF-091/092. Commit funcional inicial probado: `602459f` (`feat(m04): retire publications with preserved history`). La corrección del saldo de garantía quedó en `885bf2d`; las pruebas ampliadas de saldo y conciliación pendiente quedaron en `f9f5c98`. Issue #258 es hija de #246 y está trazada a RQF-091/092. La aceptación no completa M04 ni LOCAL-1. #246 permanece abierta.

## Comportamiento verificado

- La ruta autenticada `DELETE /api/v1/spaces/{spaceId}` exige rol de anfitrión y ownership. Un tercero recibe 404 sin revelar la publicación; un rol sin autorización recibe rechazo.
- Una publicación con fixture habilitado, reserva no terminal, operación de garantía pendiente/por conciliar o saldo autorizado sin capturar/liberar recibe `409 publication_delete_blocked`; la publicación, snapshots y reserva permanecen intactos. La prueba también establece una deducción capturada de CLP 12.000 y una liberación confirmada del saldo de CLP 38.000, con liquidación confirmada y reclamo/decisión aplicados: aunque el resumen conserve `parcialmente_capturada`, al no quedar saldo ni operación abierta la eliminación procede.
- La eliminación toma el bloqueo compartido del titular y bloquea el espacio antes de comprobar reservas, plazo de reclamo, disputas y obligaciones fake. Consulta la hora de PostgreSQL después de los bloqueos y modifica filas solo tras superar todas las precondiciones.
- Una carrera determinista colocó la eliminación delante de una solicitud de reserva basada en una cotización vigente. Ambas esperaron el bloqueo del titular; tras liberar el bloqueo la eliminación se confirmó primero y la solicitud revalidó el espacio, devolviendo conflicto sin insertar reserva ni ocupación.
- Una publicación elegible pasa a tombstone `eliminada`, desaparece de listado propio y catálogo, y su detalle de catálogo responde 404. Se retiran contenido, dirección, horario, atributos y simulaciones privadas; se eliminan fixtures deshabilitados y tarifas no referenciadas. Las tarifas necesarias para snapshots se preservan.
- Reservas terminales, ocupaciones, cotizaciones/snapshots, contratos, hechos de pago/devolución/garantía/reclamo/liquidación e historiales no se borran en cascada ni cambian de fecha. El historial de publicación sigue accesible a participantes.
- Las imágenes privadas se marcan retiradas y el worker durable elimina bytes/referencias. Un fallo sintético del primer unlink conserva el archivo y el trabajo pendiente; el siguiente intento recupera la limpieza. Los candidatos de archivo reservados también entran en la cola durable.
- El recibo idempotente y `space_deletion_history` en la exportación del titular documentan el resultado. La exportación verifica el evento de cada espacio propio sin filtrar eventos de otros titulares.

V50 mantiene el ancla técnica necesaria por las referencias históricas y agrega `espacio_eliminacion_historial_local` append-only. La clasificación de privacidad no inventa plazo de retención para ese ledger; queda pendiente de ratificación. No se declara anonimización ni supresión integral.

## Pruebas

- `GO_TEST_RUN='^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` — PASS en PostgreSQL desechable, migración desde V1 hasta V50 y operaciones con `espacigo_runtime`; demuestra saldo de garantía pendiente (saldo no asignado) y operación de liberación `por_conciliar` con saldo ya asignado como bloqueos separados. Con captura confirmada de CLP 12.000, liberación confirmada de CLP 38.000 y liquidación confirmada sin obligaciones, `parcialmente_capturada` no bloquea la eliminación. También comprueba carrera de reserva contra retiro, preservación histórica, acceso/catálogo, idempotencia y cleanup con fallo y recuperación.
- `go test ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/adapters/postgres/gallery ./internal/adapters/postgres/pricing ./internal/openapicontract ./internal/spaces/transport/http` — PASS.
- `go vet ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/adapters/postgres/gallery ./internal/adapters/postgres/pricing` — PASS.
- `npm --prefix mock run build` — PASS; TypeScript compilado y `internal/mockserver/static/app.js` actualizado.
- `npm --prefix mock run test:publication-state` — PASS, 1 prueba de rol/estado del control de publicación.
- `git diff --check` — PASS.

## Comprobación después del merge

- Tras integrar PR #263 en `main` (`a0a3ad6130745d3d2126eb4c1c92805eacc39084`), `bash scripts/dev-env.sh up -d` aplicó V50/V51 incrementalmente y backend, PostgreSQL, Mailpit y mock quedaron saludables. `bash scripts/dev-env.sh verify-http` pasó. La consulta de `schema_migrations` confirmó V47–V51 y el volumen `espacigo_pgdata` siguió presente; no se tocó el contenido de secretos ni se purgó/restauró la base.
- La integración enfocada de eliminación volvió a pasar sobre PostgreSQL desechable desde la fuente fusionada: `GO_TEST_RUN='^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces`. Cubre el conflicto cuando hay obligaciones, la eliminación cuando la garantía capturada/liberada y la liquidación están confirmadas, preservación histórica y recuperación durable de archivos.
- La liquidación/cierre pasó de nuevo en PostgreSQL desechable: `GO_TEST_RUN='^TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay$' bash scripts/test-m06-local-booking-postgres.sh`; confirma liquidación fake, estado `cerrada`, historial y replay sin duplicados.
- La regresión integral finalizada después de ambas aceptaciones está en `local-close-01-final-20261011.md`. Un hallazgo adicional de permisos de tarifa quedó corregido en el PR de seguimiento LOCAL-CLOSE-01: V52 revoca DELETE directo al runtime y permite borrar solo tarifas no referenciadas mediante una función acotada después del tombstone. V52 se integró en #264 y se aplicó incrementalmente al volumen persistente; los grants efectivos y los casos de limpieza controlada se comprobaron después del merge.

La prueba PostgreSQL es desechable y elimina su base mediante `trap`; no se tocó `espacigo_pgdata`, secretos ni datos persistentes. No se ejecutó la regresión LOCAL-1 completa: queda para después de #259.
