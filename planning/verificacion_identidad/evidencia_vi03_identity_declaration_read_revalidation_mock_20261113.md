# Evidencia VI-03 — revalidación antes de leer la declaración del titular

- **Fecha:** 2026-11-13 (`America/Santiago`).
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance:** revisión sintética local de la lectura de declaración legal en `ReviewerAccessService.LoadDetail`; no se usaron datos reales, PostgreSQL, SQL/DDL, Cloud ni proveedores externos.

## Cambio

`LoadDetail` ahora revalida la asignación exacta y la identidad/revisión del caso dentro del callback de `WithReviewerCaseForRead` antes de llamar a `LoadReviewerIdentityDeclaration`. Si la asignación falta, el servicio retorna `ErrNotFound` sin invocar el lector. La comprobación posterior del caso se conserva para descartar una declaración si cambia el vínculo mientras se carga; los errores del lector y la cancelación siguen fallando cerrados. El test positivo comprueba que una asignación vigente sí entrega el DTO allowlist.

## Prueba y resultado

Se añadió `TestReviewerIdentityDetailDoesNotReadDeclarationAfterGrantRevocation` con los fakes sintéticos ya usados por el paquete. Antes del cambio de producción, el escenario de revocación entre las dos comprobaciones falló como se esperaba: el resultado era `ErrNotFound`, pero el lector ya se había invocado una vez (`reader calls=1`, dos comprobaciones del repositorio). El control positivo pasó. No se atribuye ese RED a PostgreSQL.

Después del cambio:

- `go test -race -count=20 ./internal/documentverification -run '^TestReviewerIdentityDetailDoesNotReadDeclarationAfterGrantRevocation$' -v` — PASS (20/20).
- Suite enfocada de `LoadDetail` con `-race` — PASS.
- `go test -race -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt`, `git diff --check` para rutas existentes y revisión explícita de whitespace del archivo nuevo — PASS.

Los comandos Go se ejecutaron con entorno mínimo (`env -i`, sin configuración/variables heredadas de PostgreSQL). Revisión independiente source-only `deleg_d50ca790` aprobada (`passed=true`, sin concerns de seguridad ni errores lógicos). El revisor confirmó la comprobación del grant exacto antes del lector, el rechazo neutral sin invocación al lector y la comprobación final de grant/vínculo; recomendó mantener explícitos los límites sobre PostgreSQL, que se conservan aquí.

## Límites

La prueba simula la revocación mediante un fake y demuestra únicamente el orden de llamadas del servicio, la denegación neutral y cero lecturas del lector en ese escenario. No demuestra locks `FOR SHARE`, aislamiento, visibilidad de revocación concurrente, autorización/grants persistidos, semántica transaccional, PostgreSQL, MFA, almacenamiento ni Cloud. La autorización real y la persistencia siguen sin validarse; VI-03 y el MVP permanecen parciales.

No se hizo commit, push, deploy ni integración a `main`.
