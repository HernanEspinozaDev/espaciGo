# DOCKER-03 — recuperación y PostgreSQL sustituible

Ejecución integral aislada: **2026-10-11**, desde `scripts/test-local-docker03.sh`, con el árbol de trabajo de la rama `codex/docker-03`. La autorización del propietario permite ejecutar #270 en paralelo; **#269 continúa abierta** hasta demostrar liquidación fake tras las 24 horas, estado `cerrada`, historial, comprobante e idempotencia.

## Recursos usados

| Recurso | Nombre/ubicación de la corrida aceptada | Aislamiento |
|---|---|---|
| Proyecto Compose e imágenes | `espacigo_docker03_20261011r8` | Prefijo propio; no reemplaza tags del proyecto de desarrollo ni DOCKER-02. |
| PostgreSQL/PostGIS de trabajo | `espacigo_docker03_pgdata_20261011r8` | Instancia origen del Backend, bajo TLS. |
| PostgreSQL/PostGIS de restauración | `espacigo_docker03_restore_pgdata_20261011r8` | Cluster distinto y base restaurada `espacigo_local_restored`. |
| PostgreSQL de integraciones | `espacigo_docker03_test_pgdata_20261011r8` | Cluster separado; las pruebas crean/eliminan DB y el rol runtime sin afectar a las instancias del stack. |
| Estado privado | `~/.local/state/espacigo/docker03-20261011r8` | Directorios 0700; credenciales/backup/manifiesto 0600; CA pública montada en lectura. |
| Puertos locales | API 19080, mock 19081, Mailpit 19025, DB 19032, API restaurada 19082, probe TLS 19083 | Puertos libres dedicados a esta corrida. |

El entorno persistente de desarrollo, `espacigo_pgdata`, sus secretos y datos no se montaron ni modificaron. DOCKER-02 conserva su proyecto/volumen/secretos separados.

## Ejecución reproducible

Usar nombres nuevos para cada instalación integral. El helper rechaza nombres fuera del prefijo DOCKER-03, rutas dentro del repo o superpuestas con estado de desarrollo/DOCKER-02, puertos de las fases anteriores y usuarios runtime distintos de `espacigo_runtime`. No pasar valores secretos por la línea de comandos.

```sh
export LOCAL_DOCKER03_PROJECT_NAME=espacigo_docker03_YYYYMMDD_run1
export LOCAL_DOCKER03_STATE_DIR="$HOME/.local/state/espacigo/docker03-YYYYMMDD-run1"
export LOCAL_DOCKER03_PGDATA_VOLUME=espacigo_docker03_pgdata_YYYYMMDD_run1
export LOCAL_DOCKER03_RESTORE_VOLUME=espacigo_docker03_restore_pgdata_YYYYMMDD_run1
export LOCAL_DOCKER03_TEST_PGDATA_VOLUME=espacigo_docker03_test_pgdata_YYYYMMDD_run1
export LOCAL_DOCKER03_API_PORT=18880
export LOCAL_DOCKER03_MOCK_PORT=18881
export LOCAL_DOCKER03_MAILPIT_PORT=18825
export LOCAL_DOCKER03_DATABASE_PORT=18832
export LOCAL_DOCKER03_RESTORE_API_PORT=18882
export LOCAL_DOCKER03_TLS_PROBE_PORT=18883
scripts/local-docker03.sh config
scripts/test-local-docker03.sh
```

El test runner construye desde los targets existentes, crea sus secretos propios, levanta las dos instancias PostGIS de larga duración y ejecuta el migrador antes del Backend. Los tests de pago usan un tercer cluster; de ese modo los tests que crean y retiran el rol `espacigo_runtime` no interfieren con la API viva.

## Resultados comprobados

- **Recuperación durable real:** el runner sembró un `aviso_local` sintético con lease `procesando` expirado e intento previo persistido. Reinició el contenedor Backend completo. El worker reclamó el lease, el evento acabó `entregada` con dos intentos totales y Mailpit mostró exactamente un mensaje al destinatario fixture. No se reinició solo una función ni se simuló la recuperación con un callback manual.
- **TLS/configuración y permisos:** API y migrador usan `verify-full` con hostname del servicio y CA local. PostgreSQL reportó TLS activo. `espacigo_runtime` pudo leer datos de aplicación, no insertar en `schema_migrations` ni crear objetos; `espacigo_admin` se usa únicamente en el proceso `migrate`. La conexión con una CA local distinta falló por verificación de certificado. La API se mantuvo saludable con la CA confiable.
- **Rechazo del Backend:** una instancia temporal del Backend con el root CA alternativo respondió 503 en readiness contra el mismo PostgreSQL; se retiró el probe. El Backend normal y el restaurado se mantuvieron saludables con la CA confiable.
- **Base alternativa:** el Backend sirve desde el PostgreSQL/PostGIS dedicado `database`, definido por `DATABASE_HOST/PORT/NAME/USER`, y un Backend separado apunta al servicio `restore-database`. Se verificó lectura de readiness después de arrancar este último; la aplicación no usa la base de desarrollo como fallback. El puerto DB `19032` quedó asignado en Compose, pero la red `data` es interna y el host no puede conectar por loopback; el acceso ocurre entre servicios y las pruebas usan una IP de contenedor del cluster desechable.
- **Restore aislado:** `pg_dump` creó un archivo de 417.743 bytes con modo 0600; el restore se aplicó a una segunda instancia/volumen y base, seguida de `migrate` para asegurar checksum/esquema/grants vigentes. El dump restaurado conservó una cuenta fixture y un aviso. Los archivos privados se copiaron a un árbol distinto y sus nombres/tamaños y SHA-256 coincidieron. El helper rechazó explícitamente exponer la API antes del replay; después el sidecar se leyó/aplicó y su marcador hash validó antes de habilitar el Backend restaurado.
- **Alcance del registro externo:** en el stack limpio no había una baja completada para exportar, por lo que su sidecar contenía cero entradas y el replay operacional fue vacío. La reaplicación de una baja existente y la verificación posterior de identidad minimizada se ejercitaron con `TestLocalSuppressionReplayAfterPgDumpRestore` en la PostgreSQL desechable del runner. Por ello la evidencia separa el gate/restauración del stack del caso de replay no vacío en integración; no afirma que la corrida del stack tuviera bajas existentes.
- **Conciliación financiera:** en el cluster de pruebas desechable pasaron `TestDurableFakePaymentInboxDeduplicatesTimeoutAndRecoversAfterRestart` y `TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay`. Cubren recuperación/replay de pago fake y timeout/reinicio/replay concurrente de liquidación con efectos persistidos sin duplicar.
- **HTTP/mock:** `scripts/verify-http.py` pasó liveness/readiness, CORS/preflight y entrega de HTML, CSS, TypeScript compilado y URL pública de API del mock.
- **Comprobaciones locales de edición:** `bash -n scripts/local-docker03.sh scripts/test-local-docker03.sh`, `scripts/local-docker03.sh config` y `git diff --check` pasaron.

## Límites y seguimiento

- Esta ejecución no valida liquidación por el paso de 24 horas de DOCKER-02. #269 queda abierta hasta el recorrido de liquidación, cierre, historial, comprobante y reintento idempotente aceptado por el propietario.
- DOCKER-03 es exclusivamente local. Certificados, CA, imagen PostGIS y Mailpit son de ensayo; no demuestra Cloud SQL, GCP, SMTP productivo ni proveedor de pagos.
- Las migraciones se aplicaron en las bases aisladas de DOCKER-03. No se aplicaron migraciones nuevas al volumen persistente de desarrollo.
- Los recursos DOCKER-03 de la corrida `r8` y sus credenciales sintéticas quedan aislados para inspección. Se retiraron solo los recursos temporales de corridas anteriores creadas durante esta tarea; no se limpió el volumen de desarrollo ni se cambiaron sus secretos, y no se inspeccionaron respaldos externos desconocidos.
