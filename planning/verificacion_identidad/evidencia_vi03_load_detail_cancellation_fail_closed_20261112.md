# VI-03 — cierre seguro de `LoadDetail` ante cancelación

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido con cambios preexistentes; se conservaron sin mezclar ni revertir.

## Cambio y cobertura

`ReviewerAccessService.LoadDetail` ahora comprueba la cancelación del contexto después de que el reader de declaración devuelve y después de la revalidación del caso/grant, antes de continuar o publicar el detalle. En ambos puntos retorna `ErrDocumentStorage` neutral y `ReviewerCaseDetail{}`.

- `TestReviewerIdentityDetailFailsClosedWhenDeclarationReaderCancelsAfterReturningData`: el reader fake cancela y devuelve una declaración completa sintética. **RED observado:** el servicio seguía a una segunda lectura y devolvía la declaración con error nil. **GREEN:** rechazo inmediato, una sola lectura del repositorio y ninguna lectura de Store.
- `TestReviewerIdentityDetailFailsClosedWhenContextCancelsAfterRevalidation`: el repositorio fake cancela después del segundo callback autorizado exitoso. **RED observado:** el servicio devolvía el detalle privado con error nil. **GREEN:** respuesta vacía y error neutral.
- Ambos tests verifican un control positivo con fixture identity completo y grant exacto. Se amplió el fake de cancelación ya existente para seleccionar el número de callback; el valor predeterminado conserva el comportamiento previo de los tests de descarga.

## Verificación

Con entorno limpio (`env -i`, sin variables de DB/PG/opt-in):

- Los dos tests focales juntos — **PASS** después de observar ambos RED por la exposición del detalle.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — **PASS**.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — **PASS**.
- `go build -o /dev/null ./cmd/api` — **PASS**.
- `gofmt` y `git diff --check` — **PASS**.

## Límites

Cobertura de servicio con repositorio/reader fakes y datos sintéticos; no prueba BFF, transporte HTTP de este detalle, autorización o transacción PostgreSQL, persistencia, cancelación real de red ni descifrado. No se ejecutó SQL/DDL, se accedió a bases persistentes o Cloud, ni se usaron datos reales. Sin migraciones, commit, push, despliegue o integración a `main`.
