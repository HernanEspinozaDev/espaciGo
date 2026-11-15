# VI-03 — asignación exacta para decisiones identity

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Se añadió `TestTypedIdentityReviewRequiresExactCurrentAssignment` en `internal/documentverification/reviewer_identity_assignment_test.go`. El test usa un expediente identity sintético completo y el `memoryRepository` del paquete para pasar por `Service.ReviewIdentityWithReason`. Casos negativos: asignación ausente, scope de titular incorrecto, grant perteneciente a otra persona y capability space en un caso identity. Cada uno exige `ErrCapability` y snapshot/auditoría sin cambio. El control positivo con asignación identity exacta aprueba y avanza la revisión.

Es una caracterización test-only, PASS al primer run; no se afirma RED/GREEN ni se modificó código de producción.

## Verificación

- Test focal con `-race`: cinco escenarios PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification` — **PASS**.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — **PASS**.
- `gofmt` y `git diff --check` — limpios.

La prueba usa solo estado en memoria y datos ficticios; no se habilitaron variables de conexión ni se ejecutaron SQL/DDL, migraciones, pruebas conectadas, Cloud, proveedores externos o sitios públicos.

## Límites

No verifica grants persistidos/revocados concurrentemente, MFA, autorización efectiva, CAS/atomicidad PostgreSQL ni auditoría durable. VI-03 sigue parcial. Sin commit, push, despliegue o integración a `main`.
