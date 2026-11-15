# Evidencia VI-01/VI-02 — extracción local de ACL desde migraciones

**Fecha:** 2026-11-15
**Alcance:** corrección local del parser de ACL fuente para el historial versionado; sin operación sobre PostgreSQL ni Cloud SQL.

## Hallazgo y RED

La primera ejecución de la suite Go completa con entorno mínimo (`env -i`, sin DSN ni opt-ins) terminó con dos fallos en `internal/dbbootstrap`:

- `TestCheckedInMigrationRuntimeACLIsExtractedOrRejected`: el extractor rechazaba V85 y V95 porque una revocación incondicional y concesiones condicionadas por existencia del rol se reducían a una sola condición global.
- `TestBuildRuntimeACLContractAcceptsCheckedInMigrationHistory`: V90/V93 usaban la identidad de función `public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz)`, pero el allowlist del parser no admitía el tipo PostgreSQL `text`.

Se añadieron primero dos regresiones: asignación precisa de condición entre REVOKE/GRANT y parsing de argumentos `text`. La corrida focal observó ambos fallos esperados.

## Cambio

- El extractor ahora conserva por separado las acciones dentro de la rama reconocida y las acciones incondicionales de nivel superior; asigna la condición solo a las acciones condicionadas.
- Mantiene el rechazo fail-closed si una misma acción ACL aparece tanto dentro como fuera de la condición, o si hay efectos procedurales no condicionados junto a otros condicionados.
- El parser de identidad de funciones reconoce `text`, usado por las firmas versionadas observadas.
- No se modificó ninguna migración SQL ni el historial aplicado.

## Verificación

- Pruebas focales nuevas y regresiones de historial: `go test -race -count=1 ./internal/dbbootstrap -run 'TestExtractMigrationRuntimeACLKeepsMixedEffectConditionsExact|TestParseACLFunctionIdentitySupportsTextArguments|TestCheckedInMigrationRuntimeACLIsExtractedOrRejected|TestBuildRuntimeACLContractAcceptsCheckedInMigrationHistory' -v` — PASS, 0 FAIL.
- Caracterización de GRANT estático condicionado más GRANT incondicional del mismo tipo de acción (`TestExtractMigrationRuntimeACLRejectsStaticMixedGrants`), junto con su variante dinámica existente — PASS, rechaza el solapamiento antes de asignar condiciones.
- Suite Go completa: `env -i ... GOPROXY=off GOTOOLCHAIN=local go test -race -count=1 -json ./...` — exit 0, 0 fallos, 226 tests omitidos. Se mantuvieron desactivados los opt-ins y DSN de integración.
- `go vet ./...` — exit 0.
- `gofmt -d` y `git diff --check` en los tres archivos del parser/tests — sin diferencias ni errores.

## Revisión source-only

La primera revisión independiente (`deleg_1f8ed20b`) planteó una posible atribución errónea cuando GRANT condicionado e incondicional comparten acción. La comprobación del código y el test focalizado mostraron que `migrationACLRoleCondition` rechaza cualquier intersección de acciones top-level y condicionadas antes de asignar condiciones; también rechaza efectos procedurales incondicionales mezclados. La revisión de seguimiento (`deleg_cc9e83f9`) inspeccionó esa ruta y confirmó que no halló un caso concreto de misatribución; `logic_errors=[]`, `passed=true`. Nota de alcance: la asociación sigue siendo por nombre de acción, pero el rechazo preventivo de solapamientos es necesario y queda cubierto por las pruebas. Ambas revisiones fueron estáticas; no sustituyen pruebas SQL/runtime ni una lectura del catálogo.

## Límites

Es evidencia de análisis y pruebas de fuente, no de catálogo PostgreSQL ni de efectividad de ACL. No se consultó ni modificó ninguna base, no se ejecutó SQL/DDL, Cloud Build/Cloud Run, ni proveedor externo. VI-01/VI-02 siguen sin cerrar persistencia funcional/reinicio nativos; el gate de cleanup y la verificación actual de Cloud SQL permanecen pendientes.
