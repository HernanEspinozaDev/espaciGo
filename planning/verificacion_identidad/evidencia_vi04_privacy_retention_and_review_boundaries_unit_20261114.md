# Evidencia — VI-02 / VI-04, compuertas de retención, purga y revisión de supresión en privacy

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc4` (trabajo en worktree local).
- **Identidad de Git verificada:** `HernanEspinozaDev` (autor y committer alineados con política del repositorio).
- **Modalidad:** Test-only local con fakes en memoria (`mockReviewAndRetentionRepo`, `fakeCleaner`). Cero persistencia PostgreSQL, cero SQL/DDL, sin llamadas a servicios Cloud, sin mutación de bases persistentes, sin commit, push, deploy ni integración a `main`.

## 1. Alcance cubierto

Se implementó la suite [retention_and_review_boundaries_test.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/privacy/retention_and_review_boundaries_test.go) para verificar compuertas fail-closed, validaciones de argumentos y transiciones de retención/supresión:

1. **Perfil y solicitudes propias (`TestProfileAndOwnRequestsBoundaries`):**
   - Rechazo de `accountID` vacío devolviendo `ErrInvalid`.
   - Propagación transparente de errores de repositorio.
   - Cobertura de `Profile` y `OwnRequests` al **100,0%**.

2. **Revisión y cola de supresión (`TestSuppressionReviewAndPendingQueueBoundaries`):**
   - Validación de argumentos obligatorios en `ReviewSuppression`: rechazo fail-closed (`ErrInvalid`) ante `reviewerID` vacío, `requestID` vacío, `idempotencyKey` vacío o que exceda 200 caracteres, y `correlationID` vacío o que exceda 120 caracteres.
   - Soporte para proveedor de reloj `now` nulo (con fallback a `time.Now`) o función personalizada `func() time.Time`.
   - Propagación de error si el repositorio no implementa `SuppressionReviewRepository` o si `repo.ReviewSuppression` falla.
   - Consulta y mapeo correcto en `PendingSuppressions`.

3. **Purga de enlaces de reserva expirados (`TestPurgeExpiredReservationLinksBoundaries`):**
   - Validación de límites de paginación: rechazo con `ErrInvalid` ante `limit < 1` o `limit > 500`.
   - Conversión obligatoria de timestamp a UTC al delegar a repositorio.
   - Propagación de errores de repositorio.
   - Retorno y consolidación del resultado de purga de reservas.

4. **Purga de retenciones locales y archivos de evidencia (`TestPurgeExpiredLocalRetentionsBoundaries`):**
   - Validación estricta de parámetros: `limit < 1`, `limit > 500` o `cleaner == nil` devuelven `ErrInvalid`.
   - Manejo y propagación de fallos en preparación (`PrepareExpiredLocalRetentions`).
   - Manejo y propagación de fallos al reclamar archivos (`ClaimExpiredVerificationFiles`).
   - Manejo de fallos en el limpiador sintético de evidencias: si `cleaner.Delete(ctx, file.StorageID)` falla, el archivo se marca como diferido (`EvidenceFilesDeferred++`) y se registra en `repo.FailExpiredVerificationFile`. Si el registro de falla en repositorio produce error, se propaga inmediatamente.
   - Manejo de fallo al completar el ciclo del archivo (`CompleteExpiredVerificationFile`).
   - Acumulación de contadores en `RetentionPurgeResult` a través de múltiples lotes.

5. **Ejecución de supresión y corrida de worker (`TestExecuteSuppressionAndCleanupOnceBoundaries`):**
   - Validación de parámetros de `ExecuteSuppression`: rechazo con `ErrInvalid` ante actor, requestID, idempotencyKey o correlationID vacíos, o `cleaner == nil`.
   - Validación en `RunSuppressionCleanupOnce`: `cleaner == nil` devuelve `ErrInvalid`.
   - Manejo de colas vacías y propagación de errores al consultar pendientes en el ciclo de limpieza.

## 2. Resultados de ejecución local

```bash
$ go test -race -v -run 'Test(ProfileAndOwnRequestsBoundaries|SuppressionReviewAndPendingQueueBoundaries|PurgeExpiredReservationLinksBoundaries|PurgeExpiredLocalRetentionsBoundaries|ExecuteSuppressionAndCleanupOnceBoundaries)' ./internal/privacy
=== RUN   TestProfileAndOwnRequestsBoundaries
--- PASS: TestProfileAndOwnRequestsBoundaries (0.00s)
=== RUN   TestSuppressionReviewAndPendingQueueBoundaries
--- PASS: TestSuppressionReviewAndPendingQueueBoundaries (0.00s)
=== RUN   TestPurgeExpiredReservationLinksBoundaries
--- PASS: TestPurgeExpiredReservationLinksBoundaries (0.00s)
=== RUN   TestPurgeExpiredLocalRetentionsBoundaries
--- PASS: TestPurgeExpiredLocalRetentionsBoundaries (0.00s)
=== RUN   TestExecuteSuppressionAndCleanupOnceBoundaries
--- PASS: TestExecuteSuppressionAndCleanupOnceBoundaries (0.00s)
PASS
ok      github.com/HernanEspinozaDev/espaciGo/internal/privacy  1.575s
```

Cobertura en funciones alcanzadas de `internal/privacy/service.go`:
- `Profile`: **100,0%**
- `OwnRequests`: **100,0%**
- `ReviewSuppression`: **100,0%**
- `PendingSuppressions`: **100,0%**
- `PurgeExpiredReservationLinks`: **100,0%**
- `PurgeExpiredLocalRetentions`: **100,0%**
- `ExecuteSuppression`: **94,4%**
- `RunSuppressionCleanupOnce`: **100,0%**
- `addRetentionCounts`: **100,0%**
- Cobertura global de statements en `internal/privacy`: **62,1%** (incremento desde 46,8%).

## 3. Conformidad con directivas y límites

- `git diff --check`: limpio.
- `go vet ./internal/privacy`: limpio.
- Cero DDL, cero mutación de PostgreSQL y cero llamadas a Cloud Console / APIs remotas.
- Mantiene abiertas las compuertas de persistencia real y migración DB para fases posteriores autorizadas.