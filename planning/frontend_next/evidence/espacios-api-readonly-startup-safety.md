# Evidencia — seguridad de arranque de API para validación local READ ONLY

**Fecha:** 2026-10-27  
**Repositorio / rama:** `/home/nandev/Documentos/projects/espaciGo` / `main`  
**Alcance:** revisión estática de arranque y sonda Go one-shot; no se inició servidor HTTP ni workers. La sonda consultó PostgreSQL nativo únicamente en transacción de solo lectura; no hubo escrituras/migraciones ni se expusieron filas o agregados.

## Hallazgos de código

- `cmd/api/main.go:152-176` carga configuración y crea el pool antes de servir HTTP; la ruta de operación `migrate` es una orden CLI aparte (`main.go:132-139`), no la ruta normal de `run()`.
- Las rutas autenticadas de espacios y galería se montan dentro de `if prototypeEnabled` (`main.go:197-340`), controlado por `APP_PROFILE` / `LOCAL_AUTH_PROTOTYPE` en `cmd/api/profile.go`.
- El mismo perfil inicia workers con operaciones de limpieza/retención, entre ellas `localMediaService.CleanOnce`, `localPrivacyService.RunLocalRetentionWorker`, `localGalleryService.CleanRetiredOnce`, además de workers de identidad/notificaciones (`main.go:484-564`). Por ello, arrancar ese perfil contra una base persistente puede mutar datos aun si el navegador solo hace GET.
- `LOCAL_DATASET_DEALS_ISOLATED_CLOCK` evita parte de reconciliadores, pero su precondición es `LOCAL_BOOKING_TRIAL=1` y un archivo de reloj del dataset (`main.go:581-591`); no es un interruptor general que desactive los workers de limpieza/retención citados.
- El puerto `127.0.0.1:18081` es un destino API Docker separado del PostgreSQL nativo seleccionado en `127.0.0.1:5432`; no se usó como sustituto ni como evidencia de conexión al target nativo.

## Verificación de conexión Go — 2026-10-27

Comando: `go run ./cmd/api local-readonly-probe`, con `DATABASE_CONNECTION_MODE=direct`, host `127.0.0.1`, puerto `5432`, base `espacigo_dms_stage_20261020` y rol `espacigo_runtime`. La autenticación se obtuvo del passfile configurado para pgx; no se leyó ni imprimió su contenido.

La sonda no entra a `run()`: valida de forma cerrada el target, crea un pool pgx y ejecuta `SELECT current_database(), current_user` junto con `SELECT count(*) FROM public.espacio` dentro de una transacción `pgx.ReadOnly`. Verifica DB/rol efectivos, host/puerto, ausencia de fallbacks y credencial cargada antes de conectar. Rechaza overrides de ambiente de pgx/libpq excepto `PGPASSFILE`; nunca imprime la credencial. No devuelve ni registra el conteo. Tras ese endurecimiento se repitió el comando en entorno limpio (`env -i` con solo PATH/HOME, `PGPASSFILE` y `DATABASE_*` explícitos); salida observada: `local read-only PostgreSQL probe succeeded`. No inició listener HTTP, autenticación, servicios, workers ni operaciones de mutación.

- `go test -count=1 ./cmd/api`: PASS.
- `go vet ./cmd/api`: PASS.
- `git diff --check -- cmd/api`: PASS.
- No hubo escrituras SQL, migraciones, cambios de datos, cuentas ni sesiones.

## Decisión y límite

La conexión Go al PostgreSQL nativo quedó verificada con una sonda de solo lectura. **No iniciar el servidor API normal contra esa base persistente**: su perfil actual monta endpoints de escritura y workers de limpieza/retención. La sonda no equivale a integración HTTP/BFF, que sigue pendiente en F6. La lectura de atributos de perfil en Cloud SQL continúa denegada para el lector disponible; no se elevaron privilegios ni se usó otro destino. Esta limitación de Cloud SQL no sustituye ni invalida la comprobación local.

No se expusieron valores de filas privadas; la consulta agregada no se registró ni devolvió datos en la salida. No se modificaron SQL, permisos, DB, datos, volúmenes, infraestructura o servicios. Cambios de código de esta rebanada: `cmd/api/main.go`, `cmd/api/config.go` y la sonda/test `cmd/api/local_readonly_probe*.go`.
