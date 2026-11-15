# Evidencia VI-03 — conflicto de decisión reviewer BFF→Go por loopback sintético

- **Fecha:** 2026-11-12.
- **Backend checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Frontend checkout:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Cambio test-only:** `internal/documentverification/transport/http/identity_bff_conflict_integration_test.go` y `frontend/tooling/reviewer-decision-conflict-bff-go-integration.mjs`.

## Comportamiento caracterizado

El harness transpila y ejecuta el export `POST` real de la ruta Next BFF y el allowlist generado, sustituyendo únicamente la configuración por la URL loopback de `httptest`. Envía una decisión identity sintética completa (`needs_evidence`, revisión esperada 7, formato observado y checklist) al handler Go reviewer registrado en `http.ServeMux`. El snapshot in-memory ya está en `requiere_antecedentes`, revisión 8, con grant sintético exacto y tres archivos limpios fake; el repositorio fake rechaza la transición por revisión stale.

El BFF devuelve 409 JSON neutral (`upstream_error`, mensaje genérico y `request_id`), `private, no-store`, sin ETag ni `Set-Cookie`, sin el texto de conflicto del handler Go. En el límite Go se comprueban método/ruta/Content-Type/body exactos, Bearer sintético presente y Cookie ausente. El fake conserva estado/revisión/auditoría; observa una transición intentada y solo la carga inicial, sin readback posterior al conflicto.

Caracterización test-only al primer intento parent-verified; no se afirma RED/GREEN ni cambio productivo. Durante la preparación delegada, una primera expectativa del payload de error BFF falló; se corrigió para reflejar la respuesta neutral implementada y el resultado final se verificó de nuevo en el checkout padre.

## Verificación ejecutada en el checkout padre

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^(TestNextBFFForwardsReviewerIdentityDecisionToGoHandler|TestNextBFFSanitizesStaleReviewerIdentityDecisionConflict)$' -v` — PASS; 2 pruebas Go y 2 subpruebas Node, sin skips.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` y `go build -o /dev/null ./cmd/api` bajo `env -i` — PASS.
- `gofmt -d` y comprobaciones explícitas de whitespace en el archivo Go — limpios; `git diff --check` ejecutado.
- `node --check tooling/reviewer-decision-conflict-bff-go-integration.mjs` y `./node_modules/.bin/prettier --check tooling/reviewer-decision-conflict-bff-go-integration.mjs` — PASS; whitespace explícito en el archivo JS: cero líneas.
- Revisión independiente read-only `deleg_708e4c63`: PASS (`passed:true`, sin concerns de seguridad ni errores lógicos). Confirmó que la carga inicial ve revisión 8 y la transición fake recibe expected revision 7, falla antes del callback mutador, conserva snapshot/auditoría y no hace readback; también verificó el 409 neutral BFF sin ETag/Set-Cookie. El revisor delimitó que no cubre todo error upstream ni demuestra CAS/atomicidad PostgreSQL, runtime Next o autorización real.

## Límites

Auth, grant, repository, auditoría y estado son fakes en memoria; no prueba autorización/grants efectivos, CAS/atomicidad o rollback PostgreSQL, SQL, persistencia, runtime Next standalone, storage/GCS, Cloud ni comportamiento con datos reales. No se ejecutó SQL/DDL, no se accedió a bases persistentes o Cloud y no hubo commit, push, deploy ni integración a `main`.
