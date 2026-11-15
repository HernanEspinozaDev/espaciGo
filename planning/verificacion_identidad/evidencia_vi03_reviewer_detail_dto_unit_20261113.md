# Evidencia VI-03 — valores del DTO de detalle reviewer

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestAssignedIdentityDetailPreservesDeclaredFormatAndFileDTOValues` en `internal/documentverification/transport/http/reviewer_cases_test.go`. La prueba atraviesa el `ReviewerCasesHandler` HTTP real y el serializador de detalle con autenticación y `LoadDetail` sintéticos en memoria. El fixture usa un caso identity y metadatos de archivo ficticios.

La aserción verifica:

- HTTP 200, ETag de la revisión y `Cache-Control: private, no-store`.
- Valores exactos de `document_format_declared`, `document_format_observed` y la fecha civil observada.
- Set allowlisted del DTO de caso y ausencia del timestamp interno `document_expires_at`.
- Un archivo con set exacto de campos permitidos y valores de ID, rol, estado, tamaño, MIME y páginas.
- Los cuatro campos allowlisted de `identity_declaration`, sin campos adicionales.

Es una caracterización test-only: la primera ejecución pasó; no se observó RED y no se modificó producción. El fake representa un detalle ya autorizado; el test no evalúa grants.

## Verificación ejecutada

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification/transport/http -run '^TestAssignedIdentityDetailPreservesDeclaredFormatAndFileDTOValues$' -count=1 -v` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification/transport/http -run '^TestAssignedIdentityDetailPreservesDeclaredFormatAndFileDTOValues$' -v` — PASS, 20/20.
- `gofmt -d internal/documentverification/transport/http/reviewer_cases_test.go` — sin diferencias.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification/transport/http` — PASS.
- `git diff --check -- internal/documentverification/transport/http/reviewer_cases_test.go` — PASS.

## Límites

No prueba el `ReviewerAccessService` real, descifrado/lectura de la declaración, store privado, integración Next BFF→Go, grant o autenticación efectiva, SQL/DDL, transacción, PostgreSQL, persistencia/reinicio, GCS, Cloud ni proveedores. No sustituye los gates de persistencia de VI-03 ni cierra la integración BFF→Go. Se ejecutó solo el test unitario indicado con entorno limpio; no se ejecutaron suites de integración.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`. El worktree ya estaba sucio; se añadió únicamente la prueba y este registro, preservando el resto.