# GCP-03: ejecución del observer en scratch

Fecha: 2026-10-16. Esta evidencia corresponde al scratch `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`.

## Cambios revisados y apply

- PR #357, merge `cf209aa85372f675df3cc9837354f63cba01eee4`: lector observador con informe NDJSON acotado y pruebas locales.
- PR #358, merge `c225cbeb572732b3a1bdc5ea294a3fd62b5752c9`: Job observer separado, desactivado por defecto y con gate de Terraform independiente.
- La imagen exacta fue publicada en el Artifact Registry scratch desde el commit revisado: `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:d072139c9fbfd25476e30c6dd6ceb6552f34beaa5e960226e27ea2c55568a198`.
- Plan binario SHA-256: `bbef0971ea9e794f27e42390f34eec41f71382b63e69264b8ba16e9c16263925`. JSON SHA-256: `6d10cacbab06f0a9d0d23ec6df2433df88ff5dca4350db59d4b5b35401f73d31`. El plan exacto contenía 2 altas, 0 cambios y 0 destrucciones: el Job observer y su binding `run.invoker`. Se aplicó después de fusionar #358; los cinco Jobs de migración y permisos preexistentes quedaron sin cambios.
- El Job usa una tarea, paralelismo 1, máximo de reintentos 0, timeout 1800 s, el login migrador y la versión 4 del secreto, en la VPC privada. Su protección contra borrado quedó habilitada. El Job ejecuta consultas fijas de lectura; la sesión gestionada de Cloud SQL no impuso `default_transaction_read_only` y no se afirma ese control.

## Resultado de la única ejecución

- Ejecución `espacigo-gcp03-pilot01-observe-gpkx9`, `Completed=True`, `succeededCount=1`, índice de tarea 0 e intento 0. La lista del Job contiene una sola ejecución.
- Se recuperaron 24 registros stdout de Cloud Logging y se reordenaron por sección/índice. `ReadObserverNDJSON` aceptó el informe reconstruido: 24 registros, 236.483 bytes y SHA-256 `7ccfbb2422a87eae33038e85ed8664dd6bcb36c0132e2743297e9e1e8327c6ce`; `Passed=true`.
- Inventario observado: PostgreSQL 18.6 y PostGIS 3.6.4; extensiones `btree_gist`, `plpgsql`, `postgis` y `unaccent`; ACL runtime (16 comprobaciones) aprobadas; 377 relaciones y 1.031 funciones inspeccionadas. Las relaciones y funciones ordinarias observadas pertenecen al migrador; elementos propiedad de extensiones se identificaron separadamente.
- Las 12 funciones `SECURITY DEFINER` observadas tenían `search_path=pg_catalog, public`: ocho con EXECUTE limitado al runtime y migrador, dos helpers con EXECUTE solo para el migrador, y dos funciones trigger sin argumentos con EXECUTE para PUBLIC y el owner. Estas últimas se contrastaron con V47 y V53, que las crean como funciones trigger sin revocar ese permiso. Esto describe el estado observado y la fuente revisada; no constituye una afirmación de compatibilidad total de cada objeto.
- La prueba espacial confirmó geometría válida/intersectante, distancia geography de 110574.3885578 m y coincidencia entre índice GIST esperado y observado (válido y listo).
- No se ejecutaron migraciones, SQL manual, importaciones ni escrituras del dataset durante la observación. El informe inspecciona estado; no certifica aceptación funcional del producto.

## Cierre de credenciales: intento detenido

Después de recoger el informe, se lanzó una sola vez el Job ya existente `espacigo-gcp03-pilot01-retire` para retirar las credenciales temporales. La ejecución `espacigo-gcp03-pilot01-retire-8g8ht` falló antes de completar: la verificación de historial terminó con `permission denied for table schema_migrations` (SQLSTATE 42501). No se reintentó. No se ejecutó la revocación de roles del migrador ni la rotación/destrucción de la versión 1 del secreto owner; las credenciales permanecen pendientes de conciliación. No se deben declarar retiradas hasta reparar y revisar esta precondición.

La falla no autoriza una consulta manual ni una edición remota de permisos. El PR #359 fusionado agrega una fase fija separada, ejecutada por el migrador, que verifica las 61 versiones/checksums, roles y ACL antes de conceder solo `SELECT` sobre esta tabla a `postgres`; comprueba después que runtime y PUBLIC continúan sin lectura y que no existe `GRANT OPTION`. Esa concesión acotada se conserva como acceso a metadatos de auditoría en el scratch, porque el owner no es el grantor y no se afirma que pueda revocarla. La imagen se construyó y publicó solo en el Artifact Registry scratch: `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:10a429fdd81716a4801fbfedef5d6a889419a38fb6de1933240513b429babd30` (archivo OCI SHA-256 `0c387b44e8ab82409070113da3b77fd3c1e19d88724c63620450ba2ca4e60d2c`). El manifiesto versionado que incluye la fase y el cuarto SQL auxiliar está en PR #360. El plan Terraform privado tiene SHA binario `4c48a2f102830130738454a688972fc3b082425ad7f383e99fc51ef08e0620ad` y JSON `0585caec7ad441d3636fd93eb7db1efb789515e96552cb3f2b55f41f7e509606`; propone solo 2 altas (Job e invoker) y 3 reemplazos de las condiciones de expiración de los tres bindings IAM temporales (`16:00Z` a `18:00Z`), con 28 no-op y cero otros cambios. Está pendiente de revisión y no se aplicó. La fase nueva y una nueva ejecución de `retire` requieren revisión; `retire` no se repetirá hasta que la corrección revisada esté liberada y haya autorización expresa de ejecución.

### Reproducción local de la corrección

La integración opt-in usó un contenedor desechable PostgreSQL 18.6/PostGIS 3.6.4, sin volumen, en `127.0.0.1:55467`; se retiró al terminar. Un rol de control local separado preparó las migraciones. El login de prueba `postgres` era `NOSUPERUSER CREATEROLE`, dueño de la base pero no de `schema_migrations` (propiedad del migrador): antes del grant, la lectura reprodujo SQLSTATE 42501; tras la fase nueva leyó las 61 filas. El test comprobó que runtime y PUBLIC seguían sin SELECT y que el grant no tiene grant option.

El `retire` completo como ese owner local también reveló una diferencia: PostgreSQL vanilla rechazó `ALTER ROLE` con 42501 pese a `CREATEROLE`. Para cubrir la sintaxis del SQL de retiro y las verificaciones ACL en el contenedor, el test promovió el role desechable solo después de registrar esa limitación y completó el retiro; esto no demuestra que Cloud SQL permita la operación, ni se presenta como equivalente a su `cloudsqlsuperuser`. El intento administrado anterior no llegó a esa sentencia. Evidencia privada de integración, pruebas Go y pruebas Terraform: `.local/testing/gcp03-owner-history-fix-20261016/` (permisos 0700/0600). No se aplicó la nueva fase, plan ni cambio SQL en Cloud SQL.

## Evidencia privada

Las respuestas de Cloud Run, Cloud Logging, la reconstrucción NDJSON, el resumen validado, los metadatos de secretos y el fallo de retiro se conservan con permisos privados en `.local/testing/gcp03-observer-execution-20261016/` y `.local/testing/gcp03-close-credentials-20261016/`. No se versionan tokens ni contraseñas. La recuperación de Cloud Logging se hizo por API REST después de que el comando `gcloud logging read` fallara al decodificar la respuesta grande.

## Límites

El observer informa inventarios y comprobaciones definidas por el programa. `Passed=true` no prueba por sí solo propiedad esperada de cada objeto, compatibilidad completa, seguridad de todas las rutinas ni aceptación de producto. El proceso de retiro quedó incompleto y la protección de borrado del Job observer permanece activa. Su cleanup requiere un gate Terraform separado: primero una actualización que quite la protección del Job observer y, en una segunda etapa revisada, su eliminación; no se debe quitar la protección del SQL.
