# Evidencia VI-03 — decisión reviewer Next BFF→Go por loopback sintético

- **Fecha:** 2026-11-12.
- **Backend checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Frontend checkout autorizado:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`.
- **Ramas/HEAD:** backend `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`; frontend `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Cambio:** test-only Go `TestNextBFFForwardsReviewerIdentityDecisionToGoHandler` y harness Node `frontend/tooling/reviewer-decision-bff-go-integration.mjs`.

## Comportamiento caracterizado

El harness compila localmente el export `POST` real de la ruta BFF Next con el allowlist generado y config sustituida únicamente por la URL loopback `httptest`. Envía una decisión sintética `needs_evidence` al handler HTTP reviewer Go real registrado en mux; se ejecutan `ReviewerAccessService` y servicio de dominio reales con auth y repositorio/estado/auditoría fakes.

Se observó POST a la ruta Go reviewer exacta, JSON y body exactos, Bearer sintético preservado y Cookie ausente. La respuesta vuelve a través del BFF con 200, ETag `"8"`, `private, no-store`, DTO identity `requiere_antecedentes`, revisión 8 y formato observado `new`; no incluye el UUID del titular, `subject_id` ni storage key. El fake confirma una transición, actor/caso/revisión, estado y auditoría `needs_evidence`, además del readback.

Caracterización test-only; pasó al primer intento. No se afirma RED/GREEN ni cambio productivo.

## Verificación ejecutada

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFForwardsReviewerIdentityDecisionToGoHandler$' -v` — PASS, 1/1.
- Suites Go domain + HTTP completas con `-race -tags=bffintegration` — PASS; ambas integraciones loopback owner-upload y reviewer-decision se ejercitan en el paquete HTTP.
- `go vet` de ambos paquetes y `go build -o /dev/null ./cmd/api` — PASS.
- Frontend `pnpm lint`, `pnpm typecheck`, `pnpm test` (107/107), `pnpm api:check` (189 operaciones), `pnpm format:check` — PASS.
- `gofmt -d`, whitespace candidato y `git diff --check` — limpios.

## Revisión

La primera revisión `deleg_65ae1bbd` devolvió `passed:false` con findings sobre el body de formato/checklist, forwarding de Authorization y aserciones de respuesta. La adjudicación `deleg_bfefded6` no pudo verificarlos porque usó otro checkout y el comando terminó con «no tests to run»; se conserva como fallo de setup, no como veredicto del candidato. La re-revisión bounded read-only `deleg_5786e3b7` inspeccionó los archivos vivos untracked en los worktrees/HEAD indicados y aprobó (`passed:true`, cero concerns de seguridad/lógica). Confirmó body exacto, ruta de export Next→handler Go, forwarding Bearer/sin Cookie y verificaciones de respuesta. Sugirió afirmar el conjunto exacto de claves del DTO allowlisted; el padre añadió esa aserción para claves top-level y cada entrada de `files`, y repitió los dos tests focales (`2/2`) y las suites race — PASS. La re-revisión independiente read-only `deleg_6182d46e` aprobó el delta (`passed:true`, sin concerns de seguridad ni errores lógicos); confirmó que las claves top-level y de cada archivo coinciden con el DTO reviewer actual. Sugirió opcionalmente comprobar también el valor de `document_format_declared` y valores representativos de archivos; se conserva como mejora no bloqueante. La revisión no ejecutó tests ni modifica archivos; el padre ejecutó los dos tests focales en el checkout correcto. Límites fake-only se mantienen.

## Límites

El harness invoca la exportación del route handler Next local, no Next standalone/middleware ni runtime BFF productivo. Go utiliza `httptest`; auth es permisivo y siempre devuelve actor sintético; el grant y repository son fakes en memoria. No prueba autenticación/autorización efectiva, CAS/atomicidad PostgreSQL, SQL, persistencia, storage/GCS, Cloud ni aceptación humana. Sin SQL/DDL, bases persistentes, datos reales, commit, push, deploy o integración a `main`.
