# IMG-01 — preflight de identidad migradora

Estado: no se han aplicado V68–V74 ni la reconciliación V64. Según el operador revisor, se autorizó y ejecutó un reset del password de `postgres`; los nuevos secretos `espacigo-testprod-cloudsql-postgres-admin-password:1` y `espacigo-testprod-cloudsql-migrator-password:1` guardan credenciales sin incluirlas aquí. La conexión del login nuevo de migración pasó, pero solo con permisos read-only. Los datos siguientes son reporte del operador, no ejecución de esta tarea.

## Destino y límites

Objetivo: Cloud SQL `espacigo-testprod-db` en `southamerica-west1`, base `espacigo_dms_stage_20261020`. El operador reportó VM `espacigo-dms-bastion-v67` RUNNING en `southamerica-west1-a`, privada `10.81.2.2`, Cloud SQL RUNNABLE privada `10.189.0.15` y forward probado `127.0.0.1:15432 -> 10.189.0.15:5432`; el túnel se cerró. El mismo reporte registra PostgreSQL 18.6, ledger V1–V67 (67/67), y owners `postgres` para `usuario`, `espacio`, galerías y `schema_migrations`. El login `espacigo_migrator_login` es LOGIN restringido con CONNECT, USAGE en `public`, SELECT en ledger; sin DDL, acceso a tablas de negocio ni binding IAM nuevo al secreto.

Esta ruta incremental es un forward SSH local→bastion→Cloud SQL. No usar `dms-reverse-ssh.sh`, que pertenece al túnel inverso histórico de la importación DMS. No reiniciar DMS ni reimportar. No asumir que los datos de red permanecerán iguales: el operador los revalida al abrir una nueva sesión autorizada.

## Bloqueo actual y preflight requerido

La conexión de `espacigo_migrator_login` y la consulta read-only pasan según el operador. Su ejecución `cmd/dbmigrate -plan-only` se detuvo por `migration history says V000064 cleanup worker role is applied but a required object is absent`. No se consultaron filas de negocio ni se ejecutó DDL. Antes de cualquier escritura:

1. Confirmar `session_user`, base, instancia, versión PostgreSQL y modo read-only.
2. Comprobar el ledger `public.schema_migrations`, continuidad y hashes exactos para determinar el estado real frente a V68–V74.
3. Inventariar owners y ACL solo de los objetos relevantes a las migraciones pendientes y de la base/schema. Revisar los roles/membresías efectivos de `postgres`, `dms_migrator`, `espacigo_api_login`, `espacigo_runtime` y `espacigo_privacy_cleanup_worker`.
4. Verificar si V64 está aplicada y si el rol cleanup existe con atributos/grants esperados. Si ledger y objetos divergen, detenerse; no crear el worker ni reconstruir historial.
5. Volver a correr `cmd/dbmigrate -plan-only` tras reconciliar el drift, con destino/identidad revisados; debe producir el plan V68–V74 esperado sin otro guard.

No leer filas de negocio, contraseñas/hashes de roles ni payloads de Secret Manager. No presentar la evidencia JSON local como prueba de origen: su digest y generación GCS, si se sube, validan integridad/antigüedad del archivo, no quién conectó a SQL.

## Identidad migradora y ownership

El SQL propuesto en [reconcile_v64_cleanup_worker_role.sql](../../db/repair/reconcile_v64_cleanup_worker_role.sql) reconstituye solo el rol `espacigo_privacy_cleanup_worker` y los grants de V64 cuando se cumplen sus guards. Verifica base, `session_user=postgres`, checksum y nombre V64, rol ausente y objetos de grant. Si la rutina V64 existe, comprueba owner/seguridad y restaura su permiso EXECUTE; si falta, V73 la crea con `CREATE OR REPLACE`. No reemplaza la función, toca el ledger, reasigna owners ni modifica otros roles. Es propuesta de DBA; no se ejecutó y requiere revisión/autorización separada.

Para V68, `ALTER TABLE public.espacio ADD COLUMN` requiere ownership o autoridad equivalente; PostgreSQL no ofrece un `GRANT ALTER` de tabla. Además V68 crea FKs hacia `usuario`, `espacio` y galerías. El migrator actual no tiene DDL ni REFERENCES. Por el inventario reportado, ejecutar V68–V74 como el login migrador no es viable. Mantener ese login read-only; una ruta mínima coherente con owners actuales es una ejecución única y revisada del migrador como el propietario existente `postgres` por el forward autorizado, sin conceder `cloudsqlsuperuser` ni más membresías a `espacigo_migrator_login` o runtime. El operador debe revisar hashes/plan/backup y autorizar esa sesión admin antes de aplicar. Si esa ruta no se aprueba, se necesita un rediseño específico de ownership/permiso por objeto; no transferir ownership global.

Nunca elevar `espacigo_runtime`, `espacigo_api_login`, `espacigo_migrator_login` o el cleanup worker para resolver DDL. Mantener `SUPERUSER`, `CREATEROLE`, `CREATEDB`, `BYPASSRLS` fuera de runtime y migrator. El rol cleanup es drift y la reconciliación de arriba no se ha autorizado todavía. Si el preflight encuentra diferencia adicional, registrar operación, objeto y permiso denegado y detenerse; no compensar con grants amplios.

## Orden de migración y release

Cloud Build actual no aplica migraciones. No conectarlo con las credenciales privadas del operador ni reutilizar `_DATABASE_SECRET` de API. El procedimiento disponible depende de la sesión local/túnel y no debe convertirse en un paso de Cloud Build sin identidad de servicio, acceso a secreto y conectividad revisados por separado.

Después de validar que el nuevo secreto de `postgres` está disponible al operador autorizado, y solo con autorización específica para la reconciliación/migración:

1. Reabrir y revalidar el forward; usar el canal aprobado de secreto a archivo privado, sin valores en argumentos, variables visibles, logs o commits.
2. Ejecutar y revisar el preflight read-only. Validar el SQL exacto de reconciliación y autorización DBA; el migrador read-only no puede aplicar esta reparación.
3. Tras la aprobación, ejecutar únicamente el SQL de reconciliación como `postgres`; comprobar rol, atributos, membresías, grants y función sin consultar datos de negocio.
4. Ejecutar `-plan-only` con `espacigo_migrator_login` para revisar el plan, aunque esa identidad no tiene DDL.
5. Revisar backup, SHA y plan exacto. Para aplicar V68–V74, usar la identidad propietaria `postgres` en una ejecución independiente aprobada; detenerse ante checksum/objeto distinto.
6. Ejecutar postcheck y `-plan-only`; exigir cero pendientes y objetos/privilegios esperados antes de cualquier deploy.

Cloud Build no obtiene acceso a la sesión SSH local. No activar un job ni reutilizar el login read-only como migrator. Un futuro gate debe ordenar `plan-only → aprobación del plan/SHA → reconciliación DBA aprobada → migración como owner aprobado → postcheck`; faltan diseño de conectividad/ejecución local-túnel y aprobación de la identidad admin de esa ruta. No se añade ni activa aquí.

## Alcance de pruebas

La reconciliación propuesta se ejecutó solo contra un PostgreSQL 18 desechable y sin red: pasó la comprobación de atributos NOLOGIN/no elevados, ausencia de membresías, grants estrechos, ausencia de lectura de `usuario`/ledger y `EXECUTE` en la rutina protegida. Esto valida el SQL en fixture sintético, no el estado Cloud SQL ni la autorización para aplicarlo. También pasan los 6 tests del validador de evidencia, `validate_cloud_build_transition.py`, `bash -n` de captura, `go test ./internal/migrator ./cmd/dbmigrate` y `git diff --check`. No se ejecutaron migraciones, reparación remota, SQL remoto ni E2E online. Después de completar y revisar el esquema, el release E2E debe probar registro, correo de verificación, login y publicación con imágenes sintéticas dentro del alcance autorizado. SMTP STARTTLS fue verificado aparte según el reporte del operador, pero no está desplegado en Cloud Run; no inferir entrega de correos online hasta probarla.
