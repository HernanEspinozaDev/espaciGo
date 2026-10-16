# GCP-03: plan operativo de metadatos e IAM del ACL scratch

Fecha: 2026-10-16 (UTC). Base: `main` en merge `55e8702985656b57764452e854d9c386e9daadc7`. Proyecto explícito `espacigo-testprod` (`447373903153`), región `southamerica-west1`, raíz Terraform `infra/terraform/environments/testprod/gcp03-scratch`, state remoto `espacigo/testprod/gcp03-scratch`.

Este corte prepara un plan exacto para la identidad de la fase ACL y su secreto de propietario sin versión. No autoriza ni ejecuta `terraform apply`, SQL, migraciones, usuarios/contraseñas de Cloud SQL, versiones de Secret Manager, Jobs, dump, restore o importación. El único cambio remoto realizado en este corte fue subir la OCI ya revisada descrita abajo.

## Artefacto OCI

Se subió el archivo OCI conservado en `.local/planning/gcp/gcp-03-cloudsql-security-20261016/executor-review-bbea4e5-a/`, sin reconstruirlo ni sustituirlo. El archivo local tiene modo `0600`.

| Campo | Valor verificado |
| --- | --- |
| Commit de código fuente | `bbea4e58a02d7669ee4c5a27d8aa133542281e82` |
| Plataforma | `linux/amd64` |
| SHA-256 del archivo OCI | `33219afbd0da5a1149d2d5f11a224517a7e91d64fcce0b1fc5f9d25105079a4a` |
| Digest de manifiesto local | `sha256:cc8e99c40f698a00ddd0ab2286ee82da882281dde2b5b5f7fa25cd3bd2239f13` |
| Repositorio | `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate` |
| Tag nuevo | `source-bbea4e5` |
| Digest leído de Artifact Registry | `sha256:cc8e99c40f698a00ddd0ab2286ee82da882281dde2b5b5f7fa25cd3bd2239f13` |

El principal GCP activo se verificó contra la política IAM del proyecto; las consultas y la publicación llevaron el proyecto explícito. La identidad exacta y el recibo de `gcloud` permanecen en evidencia local privada. El tag previo `source-8fe6ef1` conserva su digest anterior `sha256:23c753939f12d035d83684094506eb0effcb20d5a1dab5544c12280a0fc461a0`.

## Plan Terraform privado

Terraform `1.16.6`, provider Google `8.7.0`; `terraform state list` leyó 11 direcciones del state remoto (8 recursos gestionados y 3 data sources). El plan binario y JSON están solo en `.local/planning/gcp/gcp-03-operation-plan-20261016/` con permisos privados. SHA-256 del plan final: `3e22f96c90d42a4c58aa98f675a38bb54071a1f0bbaf4a27ecd4bb6eaed93f89`.

Variables de seguridad del plan: `enable_scratch=true`, `enable_job=false`, `confirm_image_pushed=true`, `confirm_scratch_secret_version=false`, `confirm_database_owner_secret_version=false`, `allow_scratch_destroy=false`. Resultado: **4 crear, 0 actualizar in-place, 1 reemplazo IAM, 7 sin cambio**. El reemplazo retira el binding project-wide existente de la cuenta migradora scratch y lo cambia por un binding condicionado a la misma instancia scratch; los otros 7 recursos gestionados quedan sin cambios. Google Cloud documenta condiciones IAM para restringir `roles/cloudsql.client` al nombre y tipo de instancia ([IAM Conditions de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/iam-conditions)).

Las cuatro altas exactas son:

| Dirección | Recurso propuesto |
| --- | --- |
| `google_service_account.scratch_database_acl[0]` | Cuenta `gcp03-scratch-acl-pilot01` para la fase ACL |
| `google_project_iam_member.scratch_acl_cloudsql_client[0]` | `roles/cloudsql.client`, condicionado a `projects/espacigo-testprod/instances/espacigo-gcp03-pilot01-pg` y tipo `sqladmin.googleapis.com/Instance` |
| `google_secret_manager_secret.scratch_database_owner_password[0]` | Contenedor de secreto `espacigo-gcp03-pilot01-db-owner-password`, sin versión |
| `google_secret_manager_secret_iam_member.scratch_database_owner_access[0]` | `roles/secretmanager.secretAccessor` sobre ese secreto únicamente para la cuenta ACL |

La condición IAM del reemplazo para la cuenta `gcp03-scratch-pilot01` usa el mismo recurso Cloud SQL exacto. La política es no autoritativa (`google_project_iam_member`); el plan no reemplaza la política IAM completa del proyecto.

No hay `google_cloud_run_v2_job`, `google_sql_user`, cambios a instancia/base persistente, versión de secreto ni cambio a bindings de cuentas de aplicación en el plan. Terraform presenta una sustitución delete/create del grant migrador para añadir la condición; no se debe ejecutar un Job durante un futuro apply de esa sustitución. Una revisión o fusión del PR no autoriza aplicar el binario; cualquier apply necesita una autorización posterior y un plan vigente revisado.

## Bloqueo de permisos de V60

La prueba aislada usó PostgreSQL `18.6` en un contenedor sin red ni volumen, con un migrador `LOGIN CREATEROLE NOSUPERUSER NOCREATEDB NOINHERIT`. Crear el rol GADM restringido y ejecutar inmediatamente su `COMMENT ON ROLE` funcionó. Después de revocar la membresía temporal del migrador, la consulta encontró **cero** membresías del rol y la misma sentencia de comentario falló con SQLSTATE de permisos: `The current user must have the ADMIN option on role "espacigo_gadm_bootstrap"`. Evidencia sanitizada, imagen local y salida completa están en `.local/planning/gcp/gcp-03-operation-plan-20261016/v60-role-comment-probe.log` (modo `0600`). El ensayo no se conectó a Cloud SQL.

La secuencia propuesta actualmente crea primero `espacigo_gadm_bootstrap` en el bootstrap previo y luego reconcilia las membresías del creador; V60 comprueba que el rol no tenga membresías y al final incluye `COMMENT ON ROLE`. En PostgreSQL 18 esos dos requisitos chocan para el mismo migrador. La documentación oficial describe que el `postgres` integrado de Cloud SQL tiene `CREATEROLE`, `CREATEDB` y `LOGIN`, pero no `SUPERUSER`; PostgreSQL 18 requiere `CREATEROLE` **y `ADMIN OPTION` sobre el rol objetivo** para comentar un rol no-superusuario ([Cloud SQL: usuarios PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/users), [PostgreSQL 18: COMMENT](https://www.postgresql.org/docs/18/sql-comment.html)). No se encontró documentación que afirme que Cloud SQL omita ese requisito para `cloudsqlsuperuser`.

Por eso el plan mantiene `enable_job=false` y no propone una ruta de operador como si estuviera probada. Un operador con `CREATEROLE` y sin `ADMIN OPTION` no resuelve la sentencia; otorgarle ese `ADMIN OPTION` crea precisamente una membresía que V60 rechaza. Cloud SQL documenta `cloudsqladmin` como usuario de sistema para operaciones automatizadas, pero no documenta un mecanismo de sesión del cliente para ejecutarlo como superusuario. No se ejecutó SQL contra Cloud SQL ni se cambió V1–V61, el checksum de V60 o el atributo `NOLOGIN` del grupo.

El siguiente gate propuesto es revisar primero este conflicto y autorizar, si corresponde, una comprobación aislada en la base scratch: verificar el login, `CREATEROLE`, `rolsuper=false` y cero membresías; ejecutar únicamente el `COMMENT ON ROLE` literal de V60 como migrador; si devuelve permiso denegado, detenerse antes de correr V1–V61 y devolver evidencia sanitizada para una decisión de arquitectura. Esa comprobación aún no está autorizada ni ejecutada. Si Cloud SQL requiere un privilegio que el cliente no puede obtener sin violar la guarda de V60, no hay una ruta operativa demostrada con las restricciones actuales: será necesario volver a revisión de diseño antes de probar la migración.

## Secuencia operativa propuesta, pendiente de revisión

1. Revisar una nueva ejecución de `terraform plan` contra el state remoto y confirmar el hash. El plan actual es una fotografía revisable, no una autorización de apply. Aplicarlo requiere autorización posterior y no debe ejecutarse junto con ningún Job.
2. Si se autoriza la provisión de credenciales, usar el CLI existente sin pasar contraseñas por argumentos. Los comandos exactos apuntan a `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`:

   ```sh
   go run ./cmd/gcp03scratch-credential --apply --confirm-target=CREATE:espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch
   go run ./cmd/gcp03scratch-credential --apply --create-database-owner-credential --confirm-target=ACL-CREATE:espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch
   ```

   El primero crea la versión del secreto migrador y el usuario integrado `espacigo_scratch_migrator`; el segundo añade una versión al secreto del propietario y asigna la contraseña del usuario integrado `postgres`. Ninguno está ejecutado ni incluido en este plan. El actor operador necesitará `cloudsql.instances.get`, `cloudsql.users.create`, `cloudsql.users.update`, `cloudsql.users.get` y `cloudsql.operations.get`, más `secretmanager.versions.add` y `secretmanager.versions.destroy` sobre los dos secretos exactos. No se agrega ese operador a IAM en este PR; definir sus grants temporales exactos y comprobar su condición IAM es un gate separado. El runtime migrador conserva solo lectura de su secreto; la identidad ACL solo lee el secreto de propietario.
3. Las fases futuras de un solo paso son `bootstrap` (crea los roles `NOLOGIN`), `database_acl` (lo ejecuta `postgres`), `migrate` (V1–V61), `finalize` (verifica historial/checksums y grants) y `retire` (rota la contraseña de `postgres`). El Job y las credenciales no se preparan ahora; la fase `migrate` sigue bloqueada.
4. Después del bootstrap, el CLI de reconciliación propuesto conserva solo `cloudsqlsuperuser` en la cuenta migradora y elimina membresías de creador en roles de aplicación:

   ```sh
   go run ./cmd/gcp03scratch-credential --apply --reconcile-creator-memberships --confirm-target=RECONCILE:espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch
   ```

   La conciliación está situada después de `bootstrap` y antes de `migrate` para cumplir la guarda de membresías de V60. Hoy deja al migrador sin permiso para ejecutar el comentario de V60 en PostgreSQL 18. No ejecutar esta secuencia ni el Job de `migrate` hasta resolver y revisar el bloqueo.
5. En cualquier operación con resultado incierto, conservar las versiones secretas y consultar el estado/API exacto antes de reintentar. No destruir versiones por nombre `latest`; el CLI requiere el nombre inmutable `.../versions/N`. Al terminar una futura prueba aprobada, el retiro propuesto rota y elimina la versión owner exacta y revoca el rol Cloud SQL del migrador, verificando las respuestas antes de destruir el secreto:

   ```sh
   go run ./cmd/gcp03scratch-credential --apply --retire-database-owner-credential --confirm-target=ACL-RETIRE:espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch --secret-version=projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-db-owner-password/versions/N
   go run ./cmd/gcp03scratch-credential --apply --revoke-superuser --confirm-target=REVOKE:espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch --secret-version=projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password/versions/N
   ```

   Estas órdenes son un runbook futuro; no se ejecutaron. La retirada no borra el historial de migraciones ni cambia el checksum de V60. Un error de retiro deja el secreto para conciliación y requiere evidencia antes de otra acción.

## Evidencia y límites

- `terraform plan -refresh=true` se ejecutó contra el backend remoto de este root; solo se conservaron archivos privados de plan/state/logs.
- La OCI se cargó desde el archivo existente, se etiquetó como `source-bbea4e5` y se verificó por digest remoto; no se reconstruyó.
- Se verificó el probe local de permisos y que su contenedor se retiró al terminar.
- No se ejecutaron suites de pruebas en este corte. No se usó la base persistente ni se envió SQL a ninguna instancia Cloud SQL.
- El siguiente artefacto de Terraform, si se aprueba tras esta revisión, solo podrá planificar Jobs después de revisar el procedimiento de credenciales y resolver el bloqueo de V60.
