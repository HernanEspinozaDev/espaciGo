# Evidencia — VI-01 / VI-02, constructor de servicio, validación de cuotas e invariantes de snapshot

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit base `7d6d7c67425d0c72c918efeb10344b584d436df7`.
- **Entorno:** Local aislado Linux x86_64, Go 1.24 toolchain, PostgreSQL persistente ausente y no invocado.
- **Gate local:** Unitario/caracterización test-only con fixtures sintéticos en memoria. Sin SQL/DDL, sin acceso DB persistente ni Cloud Console / Cloud Storage.

## Verificaciones cubiertas

1. **Frontera de constructor (`NewService`):**
   - Retorno fail-closed con `ErrInvalid` ante `Repository` nulo.
   - Cobertura en `NewService`: **100,0%**.

2. **Validación de cuotas y tipos de archivo (`ValidateQuota`):**
   - Rechazo fail-closed (`ErrInvalid`) ante propósito desconocido.
   - Validación de límites de archivos: aceptación de 3 archivos en identidad y 10 en espacio; rechazo por cuota (`ErrQuota`) ante 4 archivos en identidad o 11 en espacio.
   - Detección de identificadores de archivo duplicados retornando `ErrInvalid`.
   - Rechazo fail-closed (`ErrInvalid`) ante checksum SHA256 vacío/cero.
   - Rechazo fail-closed (`ErrInvalid`) ante estados de archivo desconocidos o inválidos.
   - Rechazo por cuota (`ErrQuota`) ante tamaños de archivo <= 0 (probado con 0 y -1) o superiores al límite máximo de 10 MB (probado con 10MB+1); aceptación de exactamente 10 MB (10485760 bytes).
   - Comprobación de páginas en imágenes (JPEG/PNG): aceptación de 0 y 1 página; rechazo con `ErrInvalid` ante más de 1 página (probado con 2 páginas).
   - Comprobación de páginas en PDF: rechazo (`ErrQuota`) ante 0 páginas; aceptación de exactamente 5 páginas en identidad y rechazo ante 6 páginas; aceptación de exactamente 20 páginas en espacio y rechazo ante 21 páginas.
   - Rechazo de tipos MIME no admitidos (`text/plain`, etc.) con `ErrInvalid`.
   - Cobertura de statements en `ValidateQuota`: **100,0%**.

3. **Invariantes estructurales del caso (`validSnapshot`):**
   - Rechazo (`false`) ante propósitos inválidos.
   - Rechazo (`false`) ante identificador de snapshot o de sujeto no conforme con UUID canónico no nulo.
   - Rechazo (`false`) si un caso de identidad incluye `SpaceID`.
   - Rechazo (`false`) si un caso de espacio no incluye un `SpaceID` UUID canónico válido; aprobación (`true`) cuando el identificador es válido.
   - Rechazo (`false`) ante `SupersededAt != nil` en casos de identidad (la supersesión es exclusiva de casos de espacio); aceptación en casos de espacio.
   - Rechazo ante formatos de documento declarados u observados inválidos.
   - Rechazo ante inconsistencias en el consentimiento (versión sin timestamp o viceversa, o versión distinta a `IdentityCaptureConsentVersion`).
   - Rechazo si un caso activo carece de metadatos de declaración (`IdentityDeclarationRevisionID == ""`), o si un caso en estado terminal (`CaseRejected`, `CaseExpired`, `CaseRevoked`) retiene dicho enlace activo.
   - Rechazo si un caso de espacio incorpora metadatos identitarios (`IdentityDeclarationRevisionID`, etc.).
   - Rechazo si un archivo PDF incorpora roles de evidencia fotográfica.
   - Rechazo ante roles de evidencia duplicados o no admitidos.
   - Rechazo si se omiten roles de evidencia cuando el formato declarado de documento está fijado.
   - Rechazo ante violación de cuotas de archivo dentro del snapshot (ej. archivo sobredimensionado).
   - Cobertura de statements en `validSnapshot`: **97,5%**.

## Comprobación local ejecutada

```bash
$ go test -race -count=20 -run '^TestServiceConstructorQuotaAndSnapshotValidationBoundaries$' ./internal/documentverification
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.028s

$ go test -race -count=1 ./internal/documentverification/...
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.044s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection	15.366s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http	1.412s

$ git diff --check
# (salida limpia, 0 errores)
```

## Cobertura final obtenida

- `service.go`: `NewService` al **100,0%**, `ValidateQuota` al **100,0%**, `validSnapshot` al **97,5%**.
- Cobertura global de `internal/documentverification`: **93,5%**.
- Cobertura global de statements en todo el subsistema (`documentverification/...`): **83,6%**.

## Límites observados

- Test-only con fixtures sintéticos en memoria; sin dependencias externas de red o almacenamiento.
- Sin migraciones SQL ni ejecución DDL sobre bases de datos persistentes o Cloud SQL.
- Sin commit, push, deploy ni integración a `main`.
