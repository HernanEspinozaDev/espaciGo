# VI-03 — cancelación al terminar la lectura de evidencia

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido ampliamente dirty; se preservaron los cambios preexistentes.

## Hallazgo y cambio

`TestReviewerDownloadFailsClosedWhenContextCancelsAfterStoreRead` usa un expediente identity, grant exacto y archivo limpio sintéticos. Un control positivo con los mismos bytes/tamaño/SHA confirma que el fixture es descargable. El Store fake luego cancela el contexto inmediatamente antes de devolver bytes correctos con `nil` error. RED observado: `ReviewerAccessService.Download` devolvía los bytes y MIME con error nil pese a que el contexto ya estaba cancelado.

En `internal/documentverification/reviewer_access.go`, `Download` ahora revalida `readCtx.Err()` después de la lectura y antes de aceptar la integridad o asignar el resultado. La cancelación mantiene el error neutral `ErrDocumentStorage` y no retorna bytes ni MIME. El guard aplica a ambas ramas de propósito, identity y space.

## Verificación local

Con variables PostgreSQL/opt-in retiradas:

- Test focal antes del cambio — **RED**, respuesta contenía evidencia sintética con error nil.
- Test focal después del cambio — **PASS**.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — **PASS**.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — **PASS**.
- `go build -o /dev/null ./cmd/api` — **PASS**.
- `gofmt` y `git diff --check` en los archivos de este subcorte — **PASS**.

## Límites

Prueba unitaria del servicio con repositorio y Store fake; no acredita GCS, cifrado/almacenamiento durable, cancelación de red ni sincronización PostgreSQL. La cancelación puede ocurrir después de la última comprobación del contexto; no se afirma una exclusión atómica con el runtime. No se ejecutó SQL/DDL, se accedió a bases persistentes o Cloud, ni se usaron datos reales. Sin migraciones, commit, push, despliegue o integración a `main`.
