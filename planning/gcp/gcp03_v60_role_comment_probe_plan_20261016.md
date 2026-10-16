# GCP-03: plan de probe de COMMENT de rol en Cloud SQL

Estado: propuesta operativa para revisión. La imagen autorizada ya está publicada; todavía no se aplicaron permisos de credenciales ni Jobs y no se creó usuario, versión secreta o SQL.

## Pregunta, destino y límites

Determinar si el login PostgreSQL temporal `espacigo_scratch_migrator`, con `CREATEROLE` y sin superusuario, puede ejecutar el `COMMENT ON ROLE` requerido por el bootstrap GADM antes y después de reconciliar las membresías Cloud SQL. El destino único es proyecto `espacigo-testprod`, instancia `espacigo-gcp03-pilot01-pg`, base `espacigo_scratch`; conexión privada, PostgreSQL 18.

El programa acepta dos fases fijas. No carga archivos SQL, no ejecuta V60 ni otra migración, no toca datos de aplicación, no importa/restaura dumps y no cambia el criterio de base vacía. `role_comment_prepare` crea solo `espacigo_gadm_bootstrap NOLOGIN` con atributos privilegiados apagados y prueba el literal fijo del COMMENT. `role_comment_after_reconcile` verifica atributos exactos y cero membresías entrantes/salientes antes de repetir el mismo COMMENT. SQLSTATE `42501` es un resultado experimental válido; cualquier otro error detiene la fase.

## Preflight de esta revisión (2026-10-16)

- La instancia exacta respondió `RUNNABLE`, `POSTGRES_18`, `southamerica-west1`, con IPv4 pública desactivada y la VPC privada esperada.
- La cuenta SQL `espacigo_scratch_migrator` no aparece en `users.list`.
- El secreto metadata `espacigo-gcp03-pilot01-migrator-password` existe; `versions.list` devolvió cero versiones. No se leyó ningún payload.
- El Artifact Registry existente es `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01`. Se subió el OCI autorizado desde el commit revisado `ddd01c6b7ac5437a70453bc52ad17deaa3c96238`, etiqueta `role-comment-probe-ddd01c6`; la lectura remota confirmó digest `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`. SHA-256 del archivo OCI local: `b0cc997c1a5331ad1d8c1a36437b8b00e1c7abd789d3de96886fffe2aa988e9b`.
- Los roles personalizados `gcp03roleprobe_sqlops` y `gcp03roleprobe_secretops` no existen en el proyecto. En la política filtrada solo se vio el binding `Owner` de la identidad GCP activa; no se tomó como evidencia de comportamiento del rol temporal.
- Policy Troubleshooter está deshabilitado en el proyecto de consumo activo y no está habilitado en `espacigo-testprod`. No se habilitó la API. La llamada de lectura falló con `SERVICE_DISABLED`, por lo que no hay resultado de troubleshooting para el principal/permiso objetivo.

Repetir todas estas lecturas antes de ejecutar: el estado puede cambiar después de esta revisión.

## IAM: alcance conocido y alternativa explícita

La tabla oficial de permisos de Cloud SQL dice que los permisos del Cloud SQL Admin API se aplican al proyecto y que no se aplican permisos distintos por instancia u objeto inferior. La guía de IAM Conditions documenta condiciones basadas en instancias para operaciones concretas, pero no documenta `users.insert`, `users.get`, `users.update` ni `users.delete`. No se debe presentar la condición `resource.name == <instancia>` como comprobada para estos métodos ni usarla como una condición obligatoria que pueda denegar el probe.

La alternativa propuesta para revisión es un rol personalizado temporal vinculado al proyecto, con solo:

- `cloudsql.instances.get`
- `cloudsql.users.create`
- `cloudsql.users.delete`
- `cloudsql.users.get`
- `cloudsql.users.update`

Su binding usa únicamente `request.time < timestamp(<expiración RFC3339>)`; no afirma aislamiento por instancia. La ventana expone esas operaciones de usuarios sobre todas las instancias Cloud SQL del proyecto. Para hacer visible este alcance, `enable_role_probe_credential_permissions` requiere `confirm_project_scoped_cloudsql_user_ops=true`, está apagado por defecto y se genera en un plan separado del gate de Jobs. Revisión debe aprobar explícitamente ese riesgo acotado por permisos y expiración, o pedir otro mecanismo. Sin esa aprobación, no aplicar el gate.

El rol temporal de Secret Manager tiene `secretmanager.versions.add`, `destroy`, `get` y `list`, pero el binding se adjunta solo al secreto migrator exacto y comparte expiración. Se necesita para crear/verificar y luego retirar solo la versión identificada. La metadata del secreto ya existe; la versión se crea solo después de aprobar/aplicar el gate de credenciales y nunca se administra en Terraform.

## Secuencia operativa propuesta

1. Repetir el preflight de solo lectura del destino, historial/schema, cuenta SQL, versiones habilitadas, metadata del secreto, Jobs, políticas y digest remoto. Detenerse ante cualquier desvío del estado esperado; no leer payload del secreto.
2. Generar el plan Terraform solo con `enable_role_probe_credential_permissions=true`, `enable_role_probe_jobs=false`, identidad operador explícita, `confirm_project_scoped_cloudsql_user_ops=true` y expiración UTC próxima adecuada a la ventana. Revisar que el diff crea únicamente dos roles custom, dos bindings (Cloud SQL proyecto y Secret Manager al secreto exacto) y ningún Job. Aprobación/revisión de este plan operacional es un gate separado antes de apply.
3. Tras aprobar ese plan, aplicarlo y ejecutar una vez `gcp03scratch-credential --apply` con confirmación del destino exacto. La CLI crea únicamente el login built-in `espacigo_scratch_migrator` en la instancia y añade un password aleatorio como versión nueva al secreto existente. Anotar el número devuelto; confirmar por lectura que usuario existe y versión está `ENABLED`, sin revelar valor. No usar `latest`.
4. Crear un plan Terraform con `enable_role_probe_credential_permissions=true` (ya aplicado, conserva su expiración) y `enable_role_probe_jobs=true`, usando el digest remoto fijo anterior y la versión numérica recién creada, activa y confirmada. El delta de este plan debe limitarse a dos Jobs (una tarea, paralelismo 1, reintentos 0, timeout 600 s, 1 CPU/512 MiB), `run.invoker` por Job y `iam.serviceAccountUser` para el service account migrator. El Job gate no crea ni amplía permisos de credenciales. Revisar que no exista ningún otro cambio y aprobar este plan operacional antes de aplicar.
5. Ejecutar `role_comment_prepare` una vez. Verifica PostgreSQL 16+, identidad/base exactas, `rolsuper=false`, `rolcreaterole=true`, esquema vacío y rol prueba ausente; crea el rol fijo de grupo y prueba COMMENT.
6. Con el gate temporal de credenciales aprobado y activo dentro de su expiración, ejecutar `gcp03scratch-credential --apply --reconcile-creator-memberships` para el login exacto. Reemplaza la lista de roles por `cloudsqlsuperuser`, espera la operación y verifica la lista resultante. No modifica por SQL el rol NOLOGIN.
7. Ejecutar `role_comment_after_reconcile` una vez en una conexión nueva. Exige atributos exactos y cero membresías antes de intentar COMMENT. Si hay membresías, no ejecuta la sentencia y termina como fallo.
8. Ejecutar `--retire-role-comment-probe-login` con confirmación y versión numérica exactas. La CLI elimina el login vía Cloud SQL Admin API, espera la operación, verifica que el usuario ya no exista y luego destruye solo esa versión. Resultado de borrado incierto conserva el secreto para reintento.
9. Aplicar planes de limpieza revisados para apagar ambos gates: retirar Jobs/invocaciones/actAs y roles/bindings temporales. Verificar por lectura la ausencia de esos recursos, del login y de la versión eliminada. Conservar metadata, imagen y rol NOLOGIN de prueba en el scratch hasta su teardown existente revisado; no borrar otros secretos, usuarios, instancia, volumen ni state.
10. Registrar resultados saneados: commit/digest/version, hash del plan aplicado, IDs de operaciones/Jobs, atributos y membresías, SQLSTATE/resultado COMMENT, historial/schema intactos y cleanup IAM/Jobs/login/secreto.

## Permisos y controles

El runtime de Jobs solo usa el service account migrator, secreto de scratch, conexión privada y los métodos fijos de PostgreSQL del probe. El operador no recibe `cloudsql.admin`, Editor u Owner nuevo. La alternativa de IAM Cloud SQL temporal expone los cinco métodos de usuario listados sobre el proyecto durante la expiración; esa es una limitación/riesgo documentado, no una garantía por instancia. El permiso de Secret Manager se limita al secreto exacto. `run.invoker` se limita a cada Job y `actAs` al service account scratch. Los gates de credenciales y Jobs están separados y apagados por defecto; el gate de Jobs exige una versión numérica activa existente.

No ejecutar hasta que la revisión apruebe el alcance project-wide de IAM temporal y los dos planes operativos exactos. No aplicar Terraform, crear usuario/versión, ejecutar Jobs ni SQL como parte de la preparación de esta revisión.

## Resultado esperado y límites

La salida solo informa comportamiento del scratch administrado en esta versión/configuración. No demuestra aceptación de producto ni que el criterio de base vacía sea incorrecto. No cambia V1–V61. No ejecuta V60, migraciones, import ni restauración. La creación temporal del login/versión y las fases prepare→reconcile→after→retire requieren gates operativos revisados después de este PR.

## Referencias primarias

- [Cloud SQL PostgreSQL: permisos IAM (alcance de permisos Admin API)](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions?hl=en)
- [Cloud SQL PostgreSQL: IAM Conditions](https://docs.cloud.google.com/sql/docs/postgres/iam-conditions)
- [IAM: atributos de condiciones por recurso](https://docs.cloud.google.com/iam/docs/conditions-attribute-reference)
- [Policy Troubleshooter](https://docs.cloud.google.com/policy-intelligence/docs/troubleshoot-access)
- [PostgreSQL 18 COMMENT](https://www.postgresql.org/docs/18/sql-comment.html)
- [PostgreSQL 18 atributos de roles](https://www.postgresql.org/docs/18/role-attributes.html)
- [Cloud SQL Admin API: usuarios](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1/users)
- [Cloud SQL: reemplazar roles de usuario](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users?hl=en)
- [Cloud Run Jobs: secretos](https://docs.cloud.google.com/run/docs/configuring/jobs/secrets?hl=en)
