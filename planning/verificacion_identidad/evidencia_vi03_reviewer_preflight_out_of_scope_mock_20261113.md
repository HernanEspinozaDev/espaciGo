# Evidencia — VI-03, preflight reviewer fuera de alcance

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Alcance

`TestReviewerIdentityDecisionStopsWhenPreflightLoadIsOutOfScope` envía una solicitud sintética válida de aprobación de identidad. El fake separa `loadErr` de `ReviewIdentity`, cuya llamada cuenta de forma independiente y devolvería éxito; `Load` responde `dv.ErrNotFound`. Comprueba respuesta neutral 404, `Cache-Control: private, no-store`, ausencia de ETag, ausencia del sentinel sintético y que solo se ejecuta `Load`, sin invocar `ReviewIdentity` ni mutar argumentos de decisión. Caracteriza el cierre del handler cuando el preflight detecta un caso fuera de alcance; no cambia código productivo y no se afirma RED/GREEN.

## Verificación local

- Test focal con `-race`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`, `gofmt` y `git diff --check`: PASS.
- Cobertura de la suite HTTP: 86.8% total; `ReviewerCasesHandler.review`: 93.0% (`go test -coverprofile=...` y `go tool cover -func`).

## Revisión

La revisión source-only inicial `deleg_9c874855` encontró que el fake original compartía el error entre `Load` y `ReviewIdentity`; se corrigió usando error de preflight separado y contador independiente de llamadas a la decisión. La revisión posterior `deleg_91a4678c` informó que estaba en otro checkout/HEAD y no pudo evaluar el archivo; no se cuenta como PASS. La revisión source-only `deleg_95f55a71` aprobó el contenido actual: confirmó que `loadErr` y el contador independiente distinguen el preflight de la mutación; observó que el chequeo del sentinel es redundante con el body exacto, pero refuerza la intención de no filtrar. No ejecutó pruebas.

## Límites

El fake modela `ErrNotFound`; el test no demuestra revocación ni evaluación efectiva de grants, repositorio persistente, SQL/PostgreSQL, BFF, ni persistencia. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin cambio productivo, commit, push, deploy o integración a `main`.
