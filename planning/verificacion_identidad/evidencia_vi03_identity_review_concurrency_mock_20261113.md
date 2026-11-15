# VI-03 — decisiones concurrentes de revisor con repositorio sintético

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Se añadieron dos caracterizaciones complementarias. `TestConcurrentIdentityReviewDecisionsWithSameRevisionCommitOnce` llama al `Service` real en paralelo con dos actores/grants exactos y un fake in-memory con mutex/CAS: una decisión gana y otra devuelve `ErrConflict`. `TestConcurrentReviewerIdentityDecisionsReturnOneSuccessAndOneConflict` envía dos POST concurrentes a la ruta registrada de reviewer con el mismo `expected_revision`. La barrera del fake asegura que ambos preflight lean revisión 7 antes de decidir; el recorrido usa el handler HTTP, `ReviewerAccessService` y `Service` reales.

El fake serializa las transiciones y compara revisión antes de actualizar snapshot/auditoría. La prueba HTTP verifica exactamente una respuesta 200 con ETag de revisión 8, una respuesta 409 neutral sin ETag, dos intentos de transición, tres lecturas (dos preflight y un readback del ganador), estado final aprobado por el actor de la respuesta exitosa y una escritura de auditoría fake. Son caracterizaciones test-only que pasaron al primer run; no se afirma RED/GREEN ni se modificó producción.

## Verificación

- Test focal de dominio con `-race -count=20` — **PASS** en las 20 ejecuciones.
- Test focal HTTP con `-race -count=20` — **PASS** en las 20 ejecuciones.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification` — **PASS**.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification/transport/http` — **PASS**.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification ./internal/documentverification/transport/http` — **PASS**.
- `gofmt` y `git diff --check` — limpios.

La ejecución usó entorno limpio; no se conectó a servicios externos ni se habilitaron variables de DB/PG.

## Límites

El mutex y el CAS pertenecen al repositorio fake: esto comprueba el contrato del dominio/handler ante una revisión obsoleta y no prueba locks, serialización, CAS, atomicidad ni auditoría durable de PostgreSQL. Tampoco prueba grants persistidos/revocados, MFA, BFF, despliegue o Cloud. El criterio de concurrencia del MVP permanece abierto hasta una prueba de persistencia autorizada; no se ejecutó SQL/DDL ni consulta a bases persistentes. Sin commit, push o despliegue.
