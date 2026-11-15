# Evidencia — VI-05, backoff/cap de retry

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestRetentionRetryDelayAppliesExponentialBackoffAndCap` caracteriza la función pura `retentionRetryDelay`: base, doble en intento 2, cuádruple en intento 3, saturación al máximo cuando el siguiente doubling excedería el límite, y valor `int16` máximo sin overflow observable.
- Test-only; no cambia cálculo ni SQL/estado de retry.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Caracteriza cálculo local de duración; no prueba disponibilidad, agenda durable, persistencia, jitter, reintento real, PostgreSQL o métricas/alertas. Revisión `deleg_42cf55c6` aprobó sin errores; confirmó doubling, cap de 24 h y terminación para `int16` positivo máximo. No amplió el test a intentos/base no positivos, fuera del contrato admitido por el constructor. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
