# Evidencia — VI-01 / VI-02, transiciones de estado, consultas de titular y fronteras de servicio en Service

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit base `7d6d7c67425d0c72c918efeb10344b584d436df7`.
- **Entorno:** Local aislado Linux x86_64, Go 1.24 toolchain, PostgreSQL persistente ausente y no invocado.
- **Gate local:** Unitario/caracterización test-only con fixtures sintéticos en memoria. Sin SQL/DDL, sin acceso DB persistente ni Cloud Console / Cloud Storage.

## Verificaciones cubiertas

1. **Funciones auxiliares puras (`purposeForCap`, `reviewerScopeID`, `validReviewReason`, `validRevokeReason`, `allClean`):**
   - Retorno de cadena vacía ante capacidades o propósitos desconocidos o no parametrizados.
   - Rechazo de decisiones desconocidas o combinaciones inválidas (ej. aprobación con motivo no vacío, o `needs_evidence` con motivo de documento inválido).
   - Rechazo de motivos de revocación desconocidos.
   - `allClean` retorna `false` ante listas de archivos vacías (`nil` o `[]File{}`).
   - Cobertura de statements: `purposeForCap` al **100,0%**, `reviewerScopeID` al **100,0%**, `validReviewReason` al **100,0%**, `validRevokeReason` al **100,0%**, `allClean` al **100,0%**.

2. **Fronteras ante receptor nulo (`*Service = nil`):**
   - Retorno fail-closed con `ErrInvalid` en `ListOwnCases`, `LoadOwnCase`, `Start`, `Submit` y `AddFile`.

3. **Manejo de repositorios sin interfaz `OwnCaseReader`:**
   - Comprobación de que si el repositorio subyacente no implementa `OwnCaseReader`, `ListOwnCases` y `LoadOwnCase` retornan `ErrInvalid` de forma segura sin pánicos por aserción de tipo.

4. **Propagación de cancelación de contexto:**
   - `ListOwnCases` y `LoadOwnCase` retornan `context.Canceled` inmediatamente ante contexto cancelado.

5. **Propagación de errores de repositorio:**
   - Retorno transparente del error original devuelto por el lector de casos propios en `ListOwnCases` y `LoadOwnCase`.
   - Cobertura de statements: `ListOwnCases` al **100,0%**, `LoadOwnCase` al **100,0%**.

6. **Validaciones en `AddFile`:**
   - Rechazo fail-closed (`ErrInvalid`) ante archivos cuyo estado inicial no sea `FilePending`.
   - Rechazo fail-closed (`ErrInvalid`) ante asignación de `EvidenceRole` en casos de espacio (`PurposeSpace`).
   - Reemplazo atómico de archivo cuando ya existe un archivo con el mismo rol de evidencia en un caso de identidad con formato declarado, preservando el recuento total de archivos.
   - Cobertura de statements en `AddFile`: **87,5%**.

7. **Validaciones de transición en `Submit`:**
   - Rechazo con `ErrEvidenceIncomplete` ante casos de identidad en borrador sin archivos.
   - Rechazo con `ErrFilesNotClean` ante casos de espacio en borrador sin archivos.
   - Rechazo con `ErrEvidenceIncomplete` ante casos de identidad con archivos incompletos (ej. 2 de los 3 requeridos, faltando selfie).
   - Rechazo con `ErrFilesNotClean` ante casos con archivos que aún se encuentran en estado `FilePending`.
   - Transición exitosa a `CaseSubmitted` cuando todos los 3 archivos requeridos están en estado `FileClean`.
   - Rechazo con `ErrTransition` al intentar enviar un caso que ya no está en estado `CaseDraft` o `CaseNeedsEvidence`.
   - Transición exitosa desde `CaseNeedsEvidence` hacia `CaseSubmitted` al reenviar evidencia corregida y limpia.
   - Detección en `transact` de roles de evidencia con tipo MIME no permitido (PDF en rol de foto) que invalidan el snapshot estructural, retornando `ErrNotFound`.
   - Cobertura de statements en `Submit`: **83,3%**.

## Comprobación local ejecutada

```bash
$ go test -race -v -run '^TestServiceTransitionsAndQueryBoundaries$' ./internal/documentverification
PASS
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification   1.028s
```

```bash
$ go test -coverprofile=/tmp/cov_svc_final2.out ./internal/documentverification
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification   0.023s   coverage: 95.8% of statements
```

```bash
$ go test -race -count=1 ./internal/documentverification/...
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification              1.049s
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection   15.408s
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http 1.381s
```

## Cobertura consolidada

- `internal/documentverification/service.go`: **95,8%** de statements cubiertos.
- Cobertura consolidada del subsistema (`internal/documentverification/...`): **92,2%** de statements.
- Todos los tests unitarios ejecutados en memoria con fixtures sintéticos sin dependencias de red, base de datos ni storage real.
