# Evidencia VI-04 — ciclo local de corrección identity

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Caracterización

Se añadió `TestIdentityCorrectionReplacesPendingRoleAndResubmitsAfterSyntheticScan` usando `Service.ReviewIdentityWithReason`, `Service.AddFile` y `Service.Submit` reales con `memoryRepository`/`Authorization` sintéticos. El ciclo revisa una solicitud, registra `requiere_antecedentes` con motivo/checklist/formato/fecha observados, y permite al titular reemplazar el rol `identity_front` sin aumentar el número de slots. La nueva evidencia queda `pendiente`; el reenvío devuelve `ErrFilesNotClean` y no altera snapshot/auditoría. El test luego marca el archivo limpio directamente en el fake (no ejecuta `ScanProcessor`) y verifica reenvío con revisión vigente, estado `enviada`, revisión incrementada, reviewer despejado y evento fake de submit. La primera ejecución válida pasó: es caracterización integrada de servicios de dominio, no RED/GREEN ni cambio productivo.

## Verificación local

- Focal `-race -count=1` y repetición `-race -count=20`: PASS bajo `env -i`.
- Suites `-race`: `./internal/documentverification ./internal/documentverification/transport/http` — PASS bajo `env -i`.
- `go vet` de ambos paquetes, `go build -o /dev/null ./cmd/api`, `gofmt` y `git diff --check` — PASS.

## Revisión independiente

La revisión source-only `deleg_79a53d5a` aprobó el test sin hallazgos. Confirmó que se llaman los métodos reales del servicio y que se comprueban la decisión/auditoría sintéticas, reemplazo de rol, bloqueo de envío con evidencia pendiente y reenvío tras el estado de escaneo limpio simulado. El revisor no ejecutó pruebas. Confirmó también los límites: auditoría registrada por fake en memoria, cambio directo del estado `FileClean`, sin prueba de persistencia/atomicidad, scanner, upload, HTTP, ni base de datos.

## Subcorte adicional VI-04 — carga y `ScanProcessor` con dependencias sintéticas

Se añadió `internal/adapters/postgres/documentverification/identity_correction_scan_cycle_test.go`, `TestIdentityCorrectionResubmitsThroughRealScanProcessor`. Recorre el servicio real de decisión `requiere_antecedentes`, el `UploadService.Upload` real para reemplazar `identity_front`, el bloqueo real de `Service.Submit` mientras el archivo está pendiente, el `ScanProcessor.ProcessOne` real y el `Service.Submit` real después del resultado limpio. Usa repository, inspector, storage, jobs y scanner fakes; el scanner fake verifica que recibe exactamente los bytes guardados por la carga y devuelve metadata sintética limpia. El fake de `RecordScanOutcome` cambia a `FileClean` el archivo del mismo snapshot en memoria y aumenta la revisión antes de permitir el reenvío. Se verifica scope identity por titular/caso/archivo, tres roles únicos, auditoría fake `case_transitioned`/`enviada`, estado `CaseSubmitted` y reviewer vacío.

Caracterización test-only: la primera ejecución válida pasó; no se afirma RED/GREEN ni cambio de producción. Comandos y resultados observados bajo entorno limpio (`env -i`, sin variables PG/DATABASE):

- `go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestIdentityCorrectionResubmitsThroughRealScanProcessor$' -v` — PASS.
- `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^TestIdentityCorrectionResubmitsThroughRealScanProcessor$'` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` — PASS.
- `gofmt -d` sin diferencias y chequeo explícito de trailing whitespace — PASS.

La revisión source-only independiente `deleg_b043db97` aprobó el archivo `identity_correction_scan_cycle_test.go` (`passed=true`, sin concerns ni errores lógicos); confirmó la aserción de auditoría sintética y los límites fake-only. No ejecutó pruebas ni cubrió el test negativo añadido después.

## Ruta negativa adicional — malware bloquea el reenvío

`internal/adapters/postgres/documentverification/identity_correction_blocked_scan_cycle_test.go` añade `TestIdentityCorrectionBlockedScanCannotResubmit`. Con los mismos servicios reales y dependencias in-memory, el scanner fake devuelve `FileBlocked`; el processor produce `malware_detected` y `ReasonMalwareDetected`, el jobs fake marca bloqueada la evidencia reemplazada en el snapshot compartido, y el envío vuelve a fallar con `ErrFilesNotClean` sin mutar snapshot ni auditoría. Se verifica que el resultado no permite reenvío.

Caracterización test-only al primer run válido, sin RED/GREEN ni cambios productivos. Comandos/resultados bajo `env -i`, sin variables PG/DATABASE:

- `go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestIdentityCorrectionBlockedScanCannotResubmit$' -v` — PASS.
- `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestIdentityCorrectionResubmitsThroughRealScanProcessor|TestIdentityCorrectionBlockedScanCannotResubmit|TestProcessorForwardsCleanAndInfectedMetadata)$'` — PASS.
- `go test -race -count=1 ./internal/documentverification -run '^TestTypedIdentitySubmitRejectsEveryNonCleanRole$' -v` — 9/9 subcasos PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/documentverification` — PASS.
- `gofmt -d` sin diferencias y chequeo explícito de trailing whitespace — PASS.

La revisión source-only independiente de ambos tests, `deleg_4c4119c2`, aprobó (`passed=true`, sin concerns de seguridad ni errores lógicos). No ejecutó pruebas; su sugerencia no bloqueante confirma que el fake no valida fencing persistido, auditoría durable ni infraestructura real, límites recogidos aquí.

## Límites

El test de dominio previo usa `memoryRepository` y cambia directamente su `FileState` a limpio; por sí solo no ejercita `UploadService` ni `ScanProcessor`. Los tests nuevos llaman `UploadService` y `ScanProcessor` reales, pero repository, jobs, store y scanner siguen siendo fakes. En particular, el fake de `RecordScanOutcome` solo comprueba revisión/file ID y cambia el estado del archivo; no modela el fencing persistido de producción, validación completa de metadata ni atomicidad/auditoría durable del escaneo. No validan ClamAV, persistencia de malware/retention, cola durable, almacenamiento real, HTTP/BFF, autorización runtime, SQL/PostgreSQL, CAS durable, atomicidad ni persistencia/reinicio. Son caracterizaciones locales, no aceptación del flujo de producto.

No se ejecutó SQL/DDL, migración, consulta ni acceso a DB persistente o Cloud. Worktree compartido y cambios ajenos preservados. Sin commit, push, deploy o integración a `main`.
