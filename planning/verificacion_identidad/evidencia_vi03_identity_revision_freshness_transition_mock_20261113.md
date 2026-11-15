# Evidencia VI-03 — frescura de declaración en decisión identity

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Se añadió una defensa en `Repository.Transition`: para una decisión `review_decision` de propósito identity, el adapter consulta dentro de la transacción de transición si el ID de declaración ligado al caso sigue siendo el de mayor `revision` para el titular. Si no coincide —incluida la ausencia de revisión actual— retorna `dv.ErrConflict` antes de cargar metadata de archivos o invocar el callback de decisión. Si la consulta devuelve error, este se propaga y también se detiene antes de leer archivos.

`TestIdentityReviewRejectsCaseLinkedToOlderDeclarationRevision` cubre tres escenarios con `pgx.Tx` fake: vínculo vigente (control positivo que alcanza el callback), una declaración nueva ya presente (conflicto sin callback ni lectura de archivos) y error sintético al buscar la revisión actual (error sin callback ni lectura de archivos). En los tres se comprueba que el fake no recibe escrituras ni commit y que el flujo termina en rollback. Se actualizó `reviewerAuditFakeTx` para contestar el nuevo query en tests existentes con una revisión vigente sintética.

## Verificación local

- `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^TestIdentityReviewRejectsCaseLinkedToOlderDeclarationRevision$'` — PASS (tres escenarios por ejecución).
- `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` — PASS.
- Sensibilidad: un `go test -overlay` que retira solo la nueva guarda falla en los puntos esperados: no se llama la consulta, el caso stale alcanza el callback en vez de `ErrConflict`, y el error de lookup no detiene la transición. Esto es una mutación controlada para demostrar sensibilidad; no se presenta como RED pre-fix observado.

Todas las pruebas se ejecutaron con entorno mínimo (`env -i`), sin variables de conexión/opt-in. **No se ejecutó SQL/DDL, migración ni consulta; no se accedió a bases persistentes, Cloud, proveedores externos ni datos reales.**

## Límites

El fake modela el resultado de la consulta, no ejecuta ni valida el SQL contra PostgreSQL. Por lo tanto, esto no demuestra semántica de `ORDER BY`, aislamiento SERIALIZABLE/SSI, locks, visibilidad bajo una escritura concurrente, atomicidad decisión/auditoría, grants persistidos, ni efectos tras reinicio. La comprobación rechaza una revisión obsoleta que el lookup observe; no se afirma que por sí sola cierre el interleaving de una declaración nueva que se guarda concurrentemente con la decisión. Tampoco demuestra storage/GCS, runtime o Cloud.

El worktree compartido contiene numerosos cambios previos no pertenecientes a este subcorte; no se atribuyen aquí. Sin commit, push, despliegue o integración a `main`.

## Revisión independiente

La revisión source-only `deleg_86436ada` aprobó el delta (`passed=true`, sin hallazgos de seguridad ni errores lógicos). Recomendó no afirmar que la comprobación resuelve una declaración guardada concurrentemente: valida el estado visible en la transacción, pero no sincroniza ambos flujos. Ese límite queda explícito arriba. El revisor no ejecutó SQL ni modificó archivos.
