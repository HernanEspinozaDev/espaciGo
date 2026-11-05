# F5 — publicación sin verificación documental y límite de integración

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`, base inspeccionada `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`  
**Estado:** corte parcial verificado en PostgreSQL aislado; F5 sigue abierta.
> **Corte histórico 2026-10-28:** los hallazgos de que ClamAV no estaba disponible y de que faltaba el perfil/target F6 corresponden a ese momento. El scanner sintético ClamAV fue probado después (clean/EICAR) y F6 recibió una autorización específica con target propio; eso no cambia la persistencia pendiente F5 ni autoriza usar F6. Estado y límites actuales: `espacios-f5-documentos-flujo-arquitectura.md` y `espacios-f5-delta-persistencia-integracion-20261029.md`.
**Target de prueba:** PostgreSQL temporal del script `scripts/test-m04-attributes-postgres.sh`, datos sintéticos; no se inició `cmd/api` ni se escribieron datos en PostgreSQL nativo persistente.

## Cambios y comportamiento probado

- `internal/adapters/postgres/spaces.Repository.SetPublicationState` ya no consulta `elegibilidad_verificacion_local` ni exige elegibilidad KYC sintética para activar una publicación. Se mantienen ownership, estados/transiciones válidos, supresión/bloqueos de cuenta, zona horaria IANA y conflictos de fixtures.
- Se eliminó `spaces.ErrEligibilityRequired` y el mapeo HTTP que devolvía `eligibility_required` al publicar.
- `TestLocalPublicationAllowsUnverifiedOwnerAndRecordsOwnerTransitions` crea un usuario sintético sin elegibilidad KYC, acredita el conteo inicial en cero y publica con éxito; su flujo HTTP también confirma que la publicación solo se bloquea por ausencia de zona IANA, permite configurarla y luego publica.
- Se corrigió el conteo esperado en `TestPostgresDraftCRUDAndOwnerScope` de 8 a 9, en concordancia con el catálogo vigente de nueve perfiles usado por la matriz F3.
- El primer ciclo del test detectó una falla de migración: la función compartida `img01b_enforce_gallery_slot` evaluaba `NEW.replacement_target_space_media_id` al ejecutarse sobre el tipo de fila sintético, que no contiene esa columna. Se agregó `V000077__img01b_gallery_slot_record_guard.sql`, migración incremental que sustituye esa comprobación por lectura segura del campo desde `to_jsonb(NEW)`. V77 se ejecuta en el target temporal del test; **no** se aplicó al PostgreSQL nativo persistente.

## Evidencia ejecutada

- RED: tras retirar el gate de publicación, `TestLocalPublicationAllowsUnverifiedOwnerAndRecordsOwnerTransitions` alcanzó la inserción de galería sintética y falló con `record "new" has no field "replacement_target_space_media_id"` (SQLSTATE 42703). El fallo aisló el guard de fila en el trigger, no una conexión/fixture de publicación.
- GREEN enfocado: `GO_TEST_RUN='^TestLocalPublicationAllowsUnverifiedOwnerAndRecordsOwnerTransitions$' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` — PASS con V77 en base temporal.
- Suite de integración de espacios: `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` — PASS (5 tests).
- Unit/integration suite Go sin DSN persistente (`env -u TEST_DATABASE_URL -u DATABASE_URL -u IMG01C_TEST_DATABASE_URL -u IMG01D_TEST_DATABASE_URL -u IMG01F_TEST_DATABASE_URL go test -count=1 ./...`) — PASS.
- `go vet ./...` y `git diff --check` — PASS.
- Revisión de motor local: `clamscan`, `clamdscan`, `clamav-daemon` y una imagen local ClamAV no están disponibles.

## Límites y siguiente corte

- La prueba de reserva/contratación aún utiliza la elegibilidad KYC sintética anterior. No demuestra la nueva verificación de identidad documental, su vigencia, revocación ni bloqueo de ambas partes; no se trasladó esa elegibilidad a un distintivo.
- No existe todavía el ciclo de solicitud/documento independiente de identidad y de espacio, ACL de revisor separadas, storage privado documental, inspección real fail-closed, revisión manual, auditoría/retención ni UI/BFF/OpenAPI de documentos.
- El scanner real sigue bloqueado por falta de motor disponible; un mock no cierra F5.
- F6 sigue siendo prerrequisito para iniciar la API: falta demostrar un perfil que omite todos los workers mutadores y fuerza storage filesystem privado, además de confirmar el target aislado del recorrido browser→BFF→Go.
- No se alteró la DB persistente, se usaron documentos/cuentas reales, ni se tocó GCP, buckets, IAM, Terraform o deploy.

**Conclusión:** la compuerta de publicación KYC previa quedó retirada y el corte aislado pasa; F5 permanece parcial hasta que el modelo documental/identidad y el recorrido real autorizado se implementen y prueben.
