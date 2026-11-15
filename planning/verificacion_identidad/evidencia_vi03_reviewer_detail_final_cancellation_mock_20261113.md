# Evidencia — VI-03, cancelación al finalizar la lectura del detalle reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Alcance

`TestReviewerIdentityDetailFailsClosedWhenContextCancelsAfterLatestCaseRead` caracteriza el punto posterior al tercer read scoped en `ReviewerAccessService.LoadDetail`: después de cargar la declaración sintética, el fake cancela el contexto al devolver el snapshot más reciente. El test incluye un control positivo válido y comprueba que el escenario cancelado devuelve `ErrDocumentStorage` y detalle vacío, con tres lecturas de caso y una lectura de declaración. El test es únicamente código de prueba; no se cambió producción.

## Verificación local

- Test focal con `-race`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification`: PASS.
- `go vet ./internal/documentverification`, `gofmt` y `git diff --check`: PASS.
- Cobertura package-only, `go test -coverprofile=... ./internal/documentverification`: 84.2% total; `ReviewerAccessService.LoadDetail`: 100.0% en este paquete.

## Revisión independiente

`deleg_ce596aa0` aprobó el test en revisión source-only, sin ejecutar pruebas. Confirmó el control positivo, la cancelación después del tercer read scoped y el retorno sin detalle. Observó que el wrapper depende del contador compartido y del literal `3`; el test hace explícitos los tres reads en el control y en las aserciones.

## Límites

El repositorio y la lectura de declaración son fakes en memoria. Esto caracteriza el cierre del resultado ante cancelación en el servicio, no autorización/grants efectivos, SQL/PostgreSQL, transacción durable, BFF/HTTP, almacenamiento real, persistencia o reinicio. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin commit, push, deploy o integración a `main`.
