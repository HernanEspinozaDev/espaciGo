# Evidencia VI-03 — error tardío de frescura en cola reviewer

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Caracterización

Se amplió `reviewer_stale_declaration_list_error_test.go` con `TestReviewerQueueDiscardsEarlierCandidatesWhenLaterFreshnessLookupErrors`. El adapter real `Repository.ListReviewerCases` recibe dos candidatos y grants sintéticos con IDs, sujetos y revisiones de declaración distintos. El primero revalida su propia declaración vigente y alcanza la lectura fake de metadata de sus archivos; el segundo lookup usa el segundo par subject/revision y devuelve el error centinela. El resultado debe ser `nil` más el error original: el primer candidato acumulado no se filtra; no hay commit ni escrituras y el fake registra un rollback. Se verifican ambos pares y una traza ordenada de relectura de caso, grant, frescura y metadata, mostrando que solo el candidato uno llega a metadata.

Es una ampliación de cobertura test-only cuyo comportamiento ya estaba implementado. Pasó en la primera ejecución válida; no se afirma RED/GREEN ni hubo cambio de lógica productiva.

## Verificación local

Comandos Go ejecutados con entorno mínimo (`env -i`, conservando `PATH`, `HOME` y `TMPDIR`):

- Después de instrumentar scopes de grants, los tests `TestReviewerQueueFailsClosedWhenDeclarationFreshnessLookupErrors` y `TestReviewerQueueDiscardsEarlierCandidatesWhenLaterFreshnessLookupErrors` juntos con `-race -count=20`: PASS.
- `gofmt`, `git diff --check` y comprobación de whitespace de test/evidencia/plan: PASS.
- Suites `-race` de adapter/domain/HTTP, `go vet` de esos paquetes y `go build -o /dev/null ./cmd/api` habían pasado antes del ajuste final de la traza; no se presentan como re-ejecutados sobre esa última variación test-only.

## Límites

El adapter y `ListReviewerCases` son código real; filas, grants, lookup de frescura, archivos, commit y rollback son sintéticos. Esto caracteriza el retorno nil/error y el orden del control flow, no la semántica real de PostgreSQL, rollback durable, locks, concurrencia, grants persistidos, autorización efectiva, almacenamiento o runtime.

No se ejecutó SQL/DDL ni se accedió a una base persistente, Cloud, datos reales o servicios externos. Sin commit, push, deploy o integración a `main`.

## Revisión independiente

La revisión source-only inicial `deleg_bc401c31` encontró una debilidad de evidencia: los dos candidatos compartían subject/revision, y la relación del primer lookup con la lectura de metadata no era explícita. Se corrigió el fixture con subject y declaración distintos para el segundo candidato, se verifican ambos pares de consulta y se añade una traza ordenada por candidate ID. La revisión posterior `deleg_4baf239b` señaló que el evento de grants no mostraba los scopes devueltos. El loader real consulta el conjunto actor-wide de grants activos; por eso el fake ahora registra cada scope de las filas devueltas en cada lookup, la traza incluye esos scopes y el test exige que cada subject distinto tenga su grant identity exacto en el conjunto cargado. No se afirma aislamiento por candidato: el conjunto completo se carga para cada candidato, igual que en el adapter. La re-revisión source-only `deleg_fc15b752` aprobó sin concerns ni errores lógicos; confirmó el conjunto actor-wide y los scopes exactos, pero no ejecutó pruebas. Una primera compilación tras el cambio falló por un import faltante en el test y pasó tras añadirlo. No hubo cambio productivo ni prueba con PostgreSQL.
