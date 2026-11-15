# Evidencia — VI-04, scope del snapshot antes de cargar evidencia

- **Fecha local:** 2026-11-12 (`America/Santiago`).
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty. `upload.go` y `upload_test.go` ya contenían cambios locales de cortes anteriores; este subcorte preservó ese trabajo y añadió el guard y su regresión dentro de esos archivos.

## Brecha y cambio

`UploadService.Upload` obtenía un snapshot mediante `Service.LoadOwnCase` y usaba su propósito/IDs como scope para inspeccionar y escribir el objeto antes de que `Service.AddFile` releyera/transicionara el caso. El adapter PostgreSQL consultado en fuente filtra por ID, `subject_id` y ownership del espacio; no se probó conectado. Esta regresión simula deliberadamente una lectura inconsistente para verificar que el límite de servicio falle cerrado antes de efectos sobre la evidencia.

Inmediatamente después de `LoadOwnCase` exitoso, `UploadService.Upload` ahora devuelve `ErrNotFound` cuando el ID del snapshot no coincide con el caso pedido, el titular no coincide con el actor, un caso identity trae `SpaceID` no vacío o un caso space trae `SpaceID` que no es UUID válido. El rechazo ocurre antes de comprobar revisión/estado, inspeccionar contenido, llamar Put/Delete o transicionar metadata. Un propósito no soportado conserva el camino existente `ErrInvalid`.

`TestUploadRejectsMisScopedSnapshotBeforeInspectionOrStorage` cubre cinco casos con reader, repositorio, inspector y store sintéticos: mismatch de ID/titular, identity con espacio, y space con `SpaceID` malformado o vacío. Verifica `ErrNotFound`, recibo vacío, cero inspecciones, cero Put/Delete y cero transición/cambio de revisión, archivos o auditoría.

## TDD y verificación local

La primera corrida de la regresión retornó `ErrInvalid` porque el fake no implementaba `ListOwnCases`, requerido por la interfaz `OwnCaseReader`; se corrigió únicamente el fake, sin atribuir ese resultado al producto. Luego el padre observó RED en los cuatro casos originales: el test alcanzaba inspección/almacenamiento/transición; el caso de sujeto escribió bajo el owner sintético discrepante y el de space pasó un `SpaceID` inválido al store fake. Tras el guard esos cuatro casos pasaron. La revisión source-only inicial `deleg_6376ecfa` aprobó sin concerns ni errores lógicos y sugirió cubrir explícitamente el `SpaceID` vacío; se añadió el quinto caso después de GREEN. Su primera ejecución PASS es cobertura adicional de comportamiento existente, no se afirma RED para ese caso.

Con variables de base de datos ausentes mediante `env -i`:

- Focal `go test -race -count=5 ./internal/documentverification -run '^TestUploadRejectsMisScopedSnapshotBeforeInspectionOrStorage$' -v` — PASS, cinco ejecuciones completas de los cinco subcasos.
- `go test -race -count=1 ./internal/documentverification/...` — PASS: dominio, inspección y HTTP.
- Composición existente `TestNextBFFUploadsOwnerIdentityThroughActualUploadService`, con BFF Next exportado, ruta Go registrada, servicio real y dependencias fake: `-race -tags=bffintegration -count=5` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` en los dos archivos y `git diff --check` — PASS.
- Escaneo estático de líneas añadidas para secretos hardcodeados, shell/eval y SQL dinámico — cero coincidencias.

La re-revisión source-only `deleg_fea66392` aprobó el test final (`passed: true`, sin concerns, errores lógicos ni sugerencias). Confirmó que el quinto escenario parte de un fixture `PurposeSpace` válido, cambia solo el `SpaceID` del snapshot devuelto y alcanza el guard antes de inspección, almacenamiento o transición.

## Límites y seguridad de ejecución

Es una defensa en profundidad caracterizada con fixtures en memoria. El fake modela una lectura de metadata inconsistente; no prueba que el adapter real produzca tal snapshot. La inspección, almacenamiento y transición observados son sintéticos. No acredita grants/autorización efectiva, CAS o atomicidad PostgreSQL, durabilidad, scanner, GCS/Cloud, ni el runtime standalone de Next. El BFF loopback transpila el export/harness local y no es evidencia de runtime productivo.

No se ejecutó SQL/DDL ni se conectó a bases persistentes o Cloud. Sin datos reales, credenciales, migraciones, commit, push, integración a `main` o despliegue. VI-02/VI-04 y el MVP continúan parciales.
