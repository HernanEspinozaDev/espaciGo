# Evidencia — VI-05, validación estricta y fail-closed de configuración del worker de retención

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

En `cmd/privacy-cleanup-worker/main_test.go`, se caracterizan de forma exhaustiva las validaciones y helpers del worker en modo unitario local con datos sintéticos:

1. `TestLoadConfigRejectsInvalidOrUnsafeOptions`: cubre 20 ramas de validación fail-closed de `loadConfig` (modos de conexión `direct`/`cloudsql`, obligatoriedad de host en directo, rango de puerto, obligatoriedad y validación de archivos de clave no vacíos, allowlist SSL, pareo cert/key, backends de almacenamiento requeridos y límites de intervalo 1s-1h).
2. `TestWorkerConfigDatabaseURL`: valida el formateo exacto de la URL PostgreSQL (`postgres://user:pass@host:port/dbname`) y la codificación segura de parámetros TLS/SSL (`sslmode`, `sslrootcert`, `sslcert`, `sslkey`).
3. `TestTimedCleanerDelete`: verifica que `timedCleaner` envuelva `privacy.SyntheticEvidenceCleaner` imponiendo un deadline acotado en el contexto (verificando que `remaining <= maxDeleteDuration`), transmitiendo el ID de evidencia exacto y propagando fielmente los errores del cleaner.
4. `TestNewCleanerAndMediaStoreFilesystem`: verifica la inicialización de evidence cleaner y media store bajo directorio privado filesystem (`0o700`), el rechazo explícito ante directorios con permisos no privados (`0o777`), y el rechazo fail-closed ante backends desconocidos.

Es test-only de caracterización: pasó en su primera ejecución válida; no hubo cambio productivo ni se afirma RED/GREEN.

## Verificación local

- Focales: `go test -race -count=20 -run '^(TestLoadConfigRejectsInvalidOrUnsafeOptions|TestWorkerConfigDatabaseURL|TestTimedCleanerDelete|TestNewCleanerAndMediaStoreFilesystem)$' ./cmd/privacy-cleanup-worker` — PASS.
- Paquete completo: `go test -race -count=1 ./cmd/privacy-cleanup-worker` — PASS.
- `go vet ./cmd/privacy-cleanup-worker` — PASS.
- Cobertura total de statements en `cmd/privacy-cleanup-worker`: subió de 38,4% a 62,1% (con `databaseURL`, `Delete`, `runIdentityRetentionCycle`, `purgeDeliveredSessionOutbox` y `purgeIdentityDeclarationBatch` al 100,0%, y `loadConfig` al 95,8%).
- `gofmt -d` y `git diff --check` — limpios.

## Revisión independiente

`deleg_3cc2c2b1` aprobó source-only: confirmó las 20 ramas de validación fail-closed de `loadConfig`, el formateo seguro de URL Postgres con parámetros TLS, la delegación con timeout de `timedCleaner` y la inicialización filesystem. Sus observaciones para fortalecer la comprobación del límite temporal (`maxDeleteDuration`) y el rechazo explícito de directorios filesystem no privados (`0o777`) fueron implementadas y verificadas localmente con gates en verde. No ejecutó pruebas ni editó archivos.

## Límites

Pruebas unitarias de configuración en memoria con paths temporales. No implican conectividad a bases de datos, ejecución de procesos daemon, Cloud SQL real, Cloud Storage real ni despliegue de infraestructura. No se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
