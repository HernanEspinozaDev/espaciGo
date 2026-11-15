# Evidencia — VI-05, Put ambiguo y compensación fallida

- **Fecha:** 2026-11-13 (`America/Santiago`).
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Caracterización sintética

`TestUploadAmbiguousPutAndFailedCompensationReturnsSafeErrorWithoutMetadata` ejecuta el `UploadService.Upload` real con repository y store fake. Para el caso identity, el fake modela un `Put` que informa error después de marcar el objeto como almacenado; el `Delete` compensatorio también informa error. Se verifica que la llamada devuelve exactamente `ErrDocumentStorage` y recibo vacío, registra un Put y un Delete en el scope identity esperado, conserva `stored=true` en el modelo fake y no llama a la transición ni altera el snapshot o la auditoría del repository in-memory.

La caracterización pasó en su primera ejecución válida; no se afirma RED/GREEN y no hubo cambio de producción. Los tests vecinos ya cubrían Put ambiguo con Delete exitoso y fallo de persistencia con Delete fallido; este test cubre la combinación específica.

## Revisión y gates

Revisión source-only independiente `deleg_3a85b61d` — **PASS**, sin concerns de seguridad ni errores lógicos. Confirmó que el test alcanza `UploadService.Upload`, las copias del snapshot son suficientes para este fixture y `err == ErrDocumentStorage` coincide con el contrato actual. La revisión enfatizó que `stored=true` es solo el resultado modelado por el fake, no evidencia de persistencia en un object store real.

Con entorno limpio (`env -i`; sin variables DB/PG/opt-in):

- Focal `go test -race -count=5 ./internal/documentverification -run '^TestUploadAmbiguousPutAndFailedCompensationReturnsSafeErrorWithoutMetadata$' -v` — PASS, cinco ejecuciones.
- `go test -race -count=1 ./internal/documentverification` — PASS.
- `go vet ./internal/documentverification` — PASS.
- `gofmt` — PASS.
- Static scan de la adición — sin secretos hardcoded, shell/eval/exec ni SQL.

## Límites y gate pendiente

El código fuente devuelve `ErrDocumentStorage` si `Put` falla, intenta el Delete compensatorio y no registra un retry persistente en esa rama. Si un backend real hubiera persistido el objeto pese al error y el Delete también fallara, podría quedar un objeto sin metadata; este test **solo modela** esa posibilidad y no demuestra que ocurra en GCS u otro storage real. No prueba durabilidad, PostgreSQL, lifecycle worker, autenticación/grants, scanner ni Cloud. Una mitigación durable requeriría tracking persistente autorizado; sigue bloqueada por la prohibición vigente de SQL/DDL. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, despliegue o merge.
