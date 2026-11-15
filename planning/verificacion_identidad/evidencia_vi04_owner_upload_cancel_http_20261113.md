# Evidencia — VI-04, cancelación durante carga owner y compensación HTTP

- **Fecha:** 2026-11-13 (`America/Santiago`).
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios previos.

## Caracterización sintética

`TestOwnerHTTPUploadCompensatesSuccessfulPutAfterRequestCancellation` ejercita el `OwnerCasesHandler`, `documentverification.Service` y `UploadService` reales con auth, repository en memoria, inspector y store sintéticos. El fake store registra un `PutIdentityDocument` exitoso y cancela el contexto de la solicitud antes de retornar. El servicio de dominio no confirma la transición: la revisión permanece en 3, no se agrega archivo y el fake repository observa una carga y cero transiciones.

La carga devuelve error cancelado y el handler responde `503 temporarily unavailable`, sin ETag ni recibo. El store fake observa exactamente un Put y un Delete identity en el mismo scope subject/case/file; el Delete recibe un contexto no cancelado con deadline y el objeto simulado queda retirado. No se invocan operaciones space.

Es una caracterización de comportamiento existente; la prueba focal pasó al primer intento válido, así que no se afirma RED/GREEN ni se modificó código de producción. El test anterior que cancela una respuesta de `OwnerUploadService` fake y la prueba de dominio que verifica compensación se mantienen separados; este test une esas capas con el uploader real.

## Gates locales

Ejecución con entorno limpio (`env -i`; sin variables DB/PG/opt-in):

- Focal `go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestOwnerHTTPUploadCompensatesSuccessfulPutAfterRequestCancellation$' -v` — PASS.
- Repetición focal `-count=5` — PASS, cinco ejecuciones.
- Suites `go test -race -tags=bffintegration -count=1 ./internal/documentverification/...` — PASS en dominio, inspección y HTTP. Se configuró exclusivamente `ESPACIGO_FRONTEND_ROOT` con el checkout Next local autorizado porque otros harnesses BFF del paquete lo requieren.
- `go vet -tags=bffintegration ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS; static scan de la adición: sin credenciales hardcoded, shell/eval/exec ni SQL.

La primera ejecución de las suites sin `ESPACIGO_FRONTEND_ROOT` falló durante setup en un test BFF preexistente que exige esa variable. Se verificó directorio, TypeScript local y harness; la repetición con la ruta local autorizada pasó. No fue un fallo del test nuevo.

Revisión source-only independiente `deleg_4f8547be` — **PASS**, sin concerns de seguridad ni errores lógicos. Confirmó cancelación después del Put fake exitoso, contexto independiente con el mismo scope, respuesta 503 genérica sin recibo/ETag y ausencia de race aparente. Sugerencias opcionales, no bloqueantes: comprobar explícitamente tiempo restante positivo en el deadline y exponer/assertar el error de UploadService en un seam adicional; no cambian el alcance de esta caracterización.

## Límites

La ruta HTTP se invoca directamente en memoria; no se ejecutan Next/BFF, PostgreSQL, storage real, scanner, autenticación/grants productivos ni Cloud. El fake demuestra únicamente que la compensación del servicio usa un contexto independiente y el scope que recibió. No acredita durabilidad ni elimina el riesgo previamente documentado de un Put ambiguo seguido de fallo de Delete. No se ejecutó SQL/DDL ni se accedió a bases persistentes, servicios externos o datos reales. Sin commit, push, despliegue o merge.
