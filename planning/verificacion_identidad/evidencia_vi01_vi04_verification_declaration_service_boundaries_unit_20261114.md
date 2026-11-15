# Evidencia — VI-01 / VI-04, fronteras de constructor, receptor, guardado y decodificación en DeclarationService

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc4` (trabajo en worktree local).
- **Identidad de Git verificada:** `HernanEspinozaDev` (autor y committer alineados con política del repositorio).
- **Modalidad:** Test-only local con fakes en memoria (`declarationMemoryRepository`, `mockKeyringForBoundaries`, `mockFailingDeclarationRepo`). Cero persistencia PostgreSQL, cero SQL/DDL, sin llamadas a servicios Cloud, sin mutación de bases persistentes, sin commit, push, deploy ni integración a `main`.

## 1. Alcance cubierto

Se implementó la suite [declaration_service_boundaries_test.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/verification/declaration_service_boundaries_test.go) para ejercitar las fronteras y compuertas de `internal/verification/declaration_service.go`:

1. **Fronteras de constructor y receptor (`TestDeclarationServiceConstructorBoundaries`):**
   - Rechazo fail-closed (`ErrInvalid`) ante dependencias nulas en `NewDeclarationService` (`repo == nil`, `keys == nil`, `ids == nil`, `now == nil`, `retention == nil`).
   - Rechazo ante `consentVersion` vacía, con espacios en blanco o longitud mayor a 20 runas.
   - Validación en `NewDeclarationServiceWithRutHMAC`: rechazo ante clave secreta inferior a 16 bytes y versión de HMAC inválida (vacía, espacios, caracteres no permitidos, más de 32 caracteres, o guión inicial).
   - Comprobación de que `RutKeyring()` sobre receptor nulo (`(*DeclarationService)(nil)`) retorna `nil` de forma segura sin pánico.

2. **Fronteras de guardado (`TestDeclarationServiceSaveBoundaries`):**
   - Rechazo fail-closed (`ErrInvalid`) en `Save` ante receptor nulo (`(*DeclarationService)(nil)`).
   - Rechazo ante `accountID` no canónico (vacío, sintaxis inválida, mayúsculas, path traversal o espacios).
   - Rechazo ante consentimiento `false` o discrepancia entre `consentVersion` suministrada y la configurada en el servicio.
   - Rechazo ante múltiples correlation IDs (`len(correlationIDs) > 1`) o correlation ID inválido (vacío, espacios o > 120 runas).
   - Rechazo ante política de retención que no sea posterior a la fecha de consentimiento (`retentionAt <= at`).
   - Rechazo ante generador de UUID que falle o devuelva un identificador no canónico.
   - Propagación de error cuando `repo.CreateRevision` falla.
   - Propagación de error cuando `repo.GetRevision` de lectura post-guardado falla.
   - Rechazo con `ErrInvalid` ante cualquier discrepancia en la lectura de verificación (ID de revisión, titular, versión de esquema, versión de consentimiento o timestamp).

3. **Fronteras de consulta y decodificación (`TestDeclarationServiceGetAndDecodeBoundaries`):**
   - `GetCurrent` y `GetRevision`: rechazo fail-closed ante receptor nulo o identificadores UUID no canónicos.
   - Propagación de errores de base de datos en `GetCurrent` y `GetRevision`.
   - Retorno estricto de `ErrNotFound` cuando el registro devuelto no coincide con el `accountID` o el `id` solicitado.
   - En `decode`: rechazo con `ErrInvalid` ante `Revision <= 0` o `SchemaVersion` distinta a "1".
   - Retorno de `ErrNotFound` si la retención `RetentionAt` está expirada o es cero.
   - Manejo de fallos criptográficos en `keys.Open` devolviendo `"declaration unavailable"`.
   - Rechazo con `ErrInvalid` si el payload desencriptado contiene JSON corrupto.

## 2. Resultados de ejecución local

```bash
$ go test -race -v -run 'TestDeclarationService.*Boundaries' ./internal/verification
=== RUN   TestDeclarationServiceConstructorBoundaries
--- PASS: TestDeclarationServiceConstructorBoundaries (0.00s)
=== RUN   TestDeclarationServiceSaveBoundaries
--- PASS: TestDeclarationServiceSaveBoundaries (0.00s)
=== RUN   TestDeclarationServiceGetAndDecodeBoundaries
--- PASS: TestDeclarationServiceGetAndDecodeBoundaries (0.00s)
PASS
ok      github.com/HernanEspinozaDev/espaciGo/internal/verification  1.035s
```

Cobertura en funciones de `internal/verification/declaration_service.go`:
- `validDeclarationConsentVersion`: **100,0%**
- `validDeclarationRutHMACVersion`: **100,0%**
- `validDeclarationCorrelationID`: **100,0%**
- `NewDeclarationService`: **100,0%**
- `NewDeclarationServiceWithRutKeyring`: **77,8%**
- `NewDeclarationServiceWithRutHMAC`: **83,3%**
- `RutKeyring`: **100,0%**
- `canonicalDeclarationUUID`: **100,0%**
- `Save`: **95,5%**
- `GetCurrent`: **100,0%**
- `GetRevision`: **100,0%**
- `decode`: **92,9%**
- Cobertura de statements en `internal/verification`: **70,5%**.

## 3. Conformidad con directivas y límites

- `git diff --check`: limpio.
- `go vet ./internal/verification`: limpio.
- Cero DDL, cero mutación de PostgreSQL y cero llamadas a Cloud Console / APIs remotas.
- Mantiene abiertas las compuertas de persistencia real y migración DB para fases posteriores autorizadas.