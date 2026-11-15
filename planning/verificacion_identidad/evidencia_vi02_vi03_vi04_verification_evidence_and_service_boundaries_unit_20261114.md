# Evidencia — VI-02 / VI-03 / VI-04, fronteras de EvidenceService, SyntheticPNG y ciclo de vida de Service en verification

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc4` (worktree local).
- **Identidad de Git verificada:** `HernanEspinozaDev` (autor y committer alineados con política del repositorio).
- **Modalidad:** Test-only local con fakes en memoria (`memoryEvidenceRepo`, `memoryEvidenceStorage`, `mockVerificationRepoForBoundaries`, `generatorWithErr`). Cero persistencia PostgreSQL, cero SQL/DDL, sin llamadas a servicios Cloud, sin mutación de bases persistentes, sin commit, push, deploy ni integración a `main`.

## 1. Alcance cubierto

Se implementaron las suites unitarias [evidence_service_boundaries_test.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/verification/evidence_service_boundaries_test.go) y [service_boundaries_test.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/verification/service_boundaries_test.go) en `internal/verification`:

1. **Fronteras y compensación de EvidenceService (`TestEvidenceServiceConstructorBoundaries`, `TestEvidenceServiceUploadBoundaries`, `TestEvidenceServiceListBoundaries`, `TestEvidenceServiceContentBoundaries`, `TestEvidenceServiceDeleteBoundaries`):**
   - **Constructor fail-closed:** validación de no nulidad en dependencias (`cases`, `repo`, `store`, `ids`, `now`), retornando `ErrInvalid` si cualquiera es nil (100,0% en `NewEvidenceService`).
   - **Carga de evidencia sintética y rollback compensatorio:** validación de `owner` y `caseID` no vacíos; comprobación de existencia de caso propio vía `cases.GetOwn`; generación de PNG sintético (`SyntheticEvidenceFixture`); fallos en generador de ID o almacenamiento; y compensación crítica: si `repo.CreateEvidence` falla, se ejecuta `store.Delete(context.Background(), id)` para evitar archivos huérfanos en almacenamiento persistente.
   - **Listados de evidencia de titular y revisor:** validación de parámetros en `ListOwn` (titular y caso no vacíos, con revalidación de propiedad del caso) y `ListReview` (caso no vacío).
   - **Lectura de contenido privado y aislamiento:** validación exhaustiva de parámetros en `OwnContent` y `ReviewContent`; protección contra archivos no disponibles (`!item.FileAvailable` o `item.FileID == ""`), respondiendo unívocamente `ErrNotFound` sin consultar almacenamiento; propagación de errores del store.
   - **Eliminación de evidencia titular y revisor:** comprobación de `DeleteOwn` y `DeleteReview`: propagación de errores de borrado en store; borrado en repositorio si `FileAvailable` es falso sin invocar borrado de almacenamiento.

2. **Generación de PNG sintético fijo (`TestSyntheticPNGGeneratesValidCheckerboardPNG`):**
   - Validación estructural del fixture `SyntheticPNG()`: dimensiones exactas 96x64 px, codificación PNG válida mediante `png.DecodeConfig`.

3. **Fronteras de ciclo de vida, revocación y reintento en Service (`TestServiceOwnAndListingBoundaries`, `TestServiceRevokeBoundaries`, `TestServiceEligibilityAndHistoryBoundaries`, `TestServiceRetryComprehensiveCorrectionBoundaries`):**
   - **Casos propios y colas:** validación de identificadores en `Own` y `ListOwn`; delegación estricta a repositorio en `Pending` y `Rejected`.
   - **Revocación administrativa:** validación de identificador, revisor no vacío; allowlist estricto de motivos de revocación (`aprobacion_fixture_incorrecta`, `revision_fixture_actualizada`); longitud de clave de idempotencia acotada ([8, 200] caracteres); identificador de correlación no vacío y no superior a 120 caracteres.
   - **Elegibilidad e historial:** validación de parámetros y delegación de `Eligibility` e `History`.
   - **Matriz exhaustiva de reintento con códigos de corrección:** validación de longitud de idempotencia ([8, 80] caracteres); rechazo de reintentos para casos que no estén en estado `rechazada`; y matriz completa de validación entre motivo de rechazo y código de corrección:
     - `documento_vencido` -> únicamente `fixture_vigente_actualizado`
     - `antecedentes_incompletos` -> únicamente `antecedentes_fixture_actualizados`
     - `inicio_actividades_no_confirmado` -> únicamente `inicio_actividades_fixture_actualizadas`
     - Rechazo fail-closed de cualquier combinación cruzada o motivo desconocido.

## 2. Cobertura de statements en internal/verification

Cobertura en funciones clave de `internal/verification/evidence.go`:
- `NewEvidenceService`: **100,0%**
- `Upload`: **95,2%**
- `ListOwn`: **100,0%**
- `ListReview`: **100,0%**
- `OwnContent`: **100,0%**
- `ReviewContent`: **100,0%**
- `DeleteOwn`: **85,7%**
- `DeleteReview`: **85,7%**
- `syntheticEvidencePNG`: **90,0%**
- `SyntheticPNG`: **100,0%**

Cobertura en funciones clave de `internal/verification/service.go`:
- `Own`: **100,0%**
- `ListOwn`: **100,0%**
- `Pending`: **100,0%**
- `Rejected`: **100,0%**
- `Revoke`: **100,0%**
- `Eligibility`: **100,0%**
- `History`: **100,0%**
- `validRevocationReason`: **100,0%**
- `validCorrection`: **100,0%**

Cobertura total de statements del paquete `internal/verification`:
- Incrementó al **77,9%** (desde 70,5%).

## 3. Comandos de verificación ejecutados

```bash
# Tests focales con race detector
go test -race -v -run 'TestEvidenceService.*Boundaries|TestSyntheticPNG|TestService.*Boundaries' ./internal/verification

# Suite completa de paquetes afectados con race detector
go test -race -count=1 ./internal/verification/... ./internal/privacy/... ./internal/identityverification/... ./internal/documentverification/...

# Verificación de sintaxis y formato
git diff --check
```

Resultados:
- Todos los tests pasaron exitosamente en verde (PASS).
- `git diff --check` limpio.

## 4. Estado y compuertas pendientes

El subcorte VI-02 / VI-03 / VI-04 permanece en ámbito local con fakes en memoria a la espera de compuertas de persistencia y migración. Sin alteraciones de esquemas de bases de datos persistentes o Cloud SQL, sin SQL/DDL, sin credenciales reales ni integración a `main`.

## 5. Addendum — titularidad y fallos de borrado/compensación — 2026-11-14

Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD observado `274f37e483a078890e92382a159fa49fd23c2fe8`; worktree compartido y sucio.

Se pobló el mapa `owners` del fake en los escenarios relevantes y se añadió `TestMemoryEvidenceRepoRejectsOtherOwners`, que comprueba rechazo `ErrNotFound` en `ListOwnEvidence`, `GetOwnEvidence` y `DeleteOwnEvidence` para titular ajeno, sin eliminar el registro. `TestEvidenceServiceContentBoundaries` ahora fija también el titular y exige que `OwnContent` con otro titular no lea storage.

Se caracterizaron errores al borrar metadata en los caminos titular y revisor: el código elimina primero el blob; si el borrado de metadata falla, propaga el error y deja el registro sin blob. La prueba documenta el orden actual y **no** lo presenta como borrado durable seguro. Se añadió un caso separado donde falla `CreateEvidence` y también la compensación `store.Delete`: verifica la clave exacta, que el blob huérfano permanece y que se conserva el error original de metadata; no afirma que la compensación haya tenido éxito.

Comando ejecutado: `go test -race -count=1 ./internal/verification -run 'TestMemoryEvidenceRepoRejectsOtherOwners|TestEvidenceService(Content|Delete|UploadCompensationDeleteFailure).*|TestMemorySessionDeletionQueueBoundaries|TestHTTPDiditClientBoundaries'` — PASS. Solo pruebas seleccionadas del paquete con fakes; sin PostgreSQL, SQL/DDL, GCS/Cloud, datos reales ni recorrido browser. No equivale al cierre del ciclo durable VI-02/VI-05.