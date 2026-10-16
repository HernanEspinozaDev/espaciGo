# GCP-03: plan de probe de COMMENT de rol en Cloud SQL

Estado: probe administrado completado el 2026-10-16; PR #352 aprobado y fusionado. Esta prueba no ejecutó V60 ni V1–V61 y no cambia su checksum ni el criterio de base vacía.

## Pregunta, destino y límites

Determinar si el login PostgreSQL temporal `espacigo_scratch_migrator`, con `CREATEROLE` y sin superusuario, puede ejecutar el `COMMENT ON ROLE` requerido por el bootstrap GADM antes y después de reconciliar las membresías Cloud SQL. El destino único es proyecto `espacigo-testprod`, instancia `espacigo-gcp03-pilot01-pg`, base `espacigo_scratch`; conexión privada, PostgreSQL 18.

El programa acepta dos fases fijas. No carga archivos SQL, no ejecuta V60 ni otra migración, no toca datos de aplicación, no importa/restaura dumps y no cambia el criterio de base vacía. `role_comment_prepare` crea solo `espacigo_gadm_bootstrap NOLOGIN` con atributos privilegiados apagados y prueba el literal fijo del COMMENT. `role_comment_after_reconcile` verifica atributos exactos y cero membresías entrantes/salientes antes de repetir el mismo COMMENT. SQLSTATE `42501` es un resultado experimental válido; cualquier otro error detiene la fase.

## Preflight anterior a credenciales y Jobs (2026-10-16)

- La instancia exacta respondió `RUNNABLE`, `POSTGRES_18`, `southamerica-west1`, con IPv4 pública desactivada y la VPC privada esperada.
- La cuenta SQL `espacigo_scratch_migrator` no aparece en `users.list`.
- El secreto metadata `espacigo-gcp03-pilot01-migrator-password` existe; `versions.list` devolvió cero versiones. No se leyó ningún payload.
- El Artifact Registry existente es `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01`. Se subió el OCI autorizado desde el commit revisado `ddd01c6b7ac5437a70453bc52ad17deaa3c96238`, etiqueta `role-comment-probe-ddd01c6`; la lectura remota confirmó digest `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`. SHA-256 del archivo OCI local: `b0cc997c1a5331ad1d8c1a36437b8b00e1c7abd789d3de96886fffe2aa988e9b`.
- Los roles personalizados `gcp03roleprobe_sqlops` y `gcp03roleprobe_secretops` no existen en el proyecto. En la política filtrada solo se vio el binding `Owner` de la identidad GCP activa; no se tomó como evidencia de comportamiento del rol temporal.
- Policy Troubleshooter está deshabilitado en el proyecto de consumo activo y no está habilitado en `espacigo-testprod`. No se habilitó la API. La llamada de lectura falló con `SERVICE_DISABLED`, por lo que no hay resultado de troubleshooting para el principal/permiso objetivo.

Este preflight es histórico, anterior al usuario/versión/Jobs temporales, y no describe su estado actual. El retiro posterior del usuario/version se documenta en [resultado administrado](gcp03_role_probe_execution_20261016.md).

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

## Resultado administrado (2026-10-16)

- La versión migrador 3 se habilitó para el ensayo y luego fue destruida exactamente; el login `espacigo_scratch_migrator` se retiró por Cloud SQL Admin API y se confirmó ausente.
- El Job `role_comment_prepare` se ejecutó una vez. PostgreSQL informó `180006`; el login era `CREATEROLE`, no superusuario; el grupo tenía todos sus atributos privilegiados/login en false y cero membresías. El comentario fijo tuvo resultado `succeeded`, SQLSTATE vacío.
- La operación `UPDATE_USER` `e928ae45-bf9a-4789-94d0-3c690000004b` terminó `DONE`, conservando solo `cloudsqlsuperuser`.
- `role_comment_after_reconcile` se ejecutó una vez, confirmó atributos seguros y cero membresías, y el mismo comentario tuvo resultado `succeeded`, SQLSTATE vacío.
- La operación `DELETE_USER` `35e2e7b6-2da2-4b89-b429-78940000004b` terminó `DONE`; la versión 3 quedó `DESTROYED`.
- Los Jobs no ejecutaron migraciones, no importaron datos ni alteraron tablas de aplicación. Las únicas sentencias SQL fueron crear el rol de prueba y los dos comentarios fijos.

La hipótesis local que esperaba `42501` no ocurrió en la instancia/configuración administrada. Esto demuestra únicamente que el literal COMMENT pasó en esas dos sesiones de prueba. No ejecutó el cuerpo completo de V60: sus guards, grants, trigger, ownership, ACL/runtime y la cadena V1–V61 aún requieren el ensayo separado descrito en [alcance de replay](gcp03_cloudsql_v1_v61_replay_scope_20261016.md). El ADR baseline sigue propuesto.

## Permisos y controles

El runtime de Jobs solo usa el service account migrator, secreto de scratch, conexión privada y los métodos fijos de PostgreSQL del probe. El operador no recibe `cloudsql.admin`, Editor u Owner nuevo. La alternativa de IAM Cloud SQL temporal expone los cinco métodos de usuario listados sobre el proyecto durante la expiración; esa es una limitación/riesgo documentado, no una garantía por instancia. El permiso de Secret Manager se limita al secreto exacto. `run.invoker` se limita a cada Job y `actAs` al service account scratch. Los gates de credenciales y Jobs están separados y apagados por defecto; el gate de Jobs exige una versión numérica activa existente.

El plan de credenciales/Jobs del probe fue aplicado con autorización temporal y ya se retiraron el login y la versión. Quedan Jobs/IAM temporales para cleanup; ver [planes cleanup](gcp03_role_probe_cleanup_plan_20261016.md). No hay autorización vigente para ejecutar el flujo completo de migración.

## Resultado esperado y límites

La salida solo informa comportamiento del scratch administrado en esta versión/configuración. No demuestra aceptación de producto ni que el criterio de base vacía sea incorrecto. No cambia V1–V61. No ejecuta V60, migraciones, import ni restauración. El flujo prepare→reconcile→after→retire quedó completado y registrado; el replay V1–V61 sigue pendiente de planes y revisión específicos.

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
