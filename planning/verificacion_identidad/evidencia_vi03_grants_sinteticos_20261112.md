# VI-03 — autorización de grants reviewer con fixtures

- Fecha: 2026-11-12.
- Checkout/rama/HEAD: `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- El worktree estaba compartido y ampliamente dirty; no se alteraron los cambios ajenos.

## Cobertura agregada

Se añadió `TestReviewerGrantMatchesRequiresExactPurposeScope` para el helper puro `reviewerGrantMatches` del adapter. Casos sintéticos cubren grant exacto identity y space; scope equivocado; reviewer distinto; mezcla de capacidades identity/space; concesión adicional de identity que permanece limitada por scope; auto-revisión; caso reemplazado; propósito desconocido y grant malformado. Es caracterización de la política local ya implementada; no se modificó producción.

## Verificación

Se retiraron explícitamente variables de conexión PostgreSQL y de opt-in de pruebas de integración:

- `go test -race ./internal/adapters/postgres/documentverification -run '^TestReviewerGrantMatchesRequiresExactPurposeScope$' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` — PASS.

El test ejecuta el comparador en memoria, sin `pgxpool`, consulta, transacción o SQL. No se ejecutó la suite integral del adapter porque contiene pruebas de integración opt-in.

## Límites

No demuestra grants leídos de PostgreSQL, expiración/revocación bajo bloqueo, acceso HTTP integrado, persistencia o MFA. La condición de seguridad de grants activos se caracteriza solo en el comparador; la lectura filtrada y concurrencia durable siguen requiriendo sus gates no autorizados aquí. No hubo SQL/DDL, base persistente, Cloud, datos reales, publicación ni integración a `main`.
