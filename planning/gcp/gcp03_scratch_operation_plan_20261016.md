# GCP-03: plan operativo de metadatos e IAM del ACL scratch

Documento de preparación inicial (2026-10-16 UTC), basado en `main` merge `55e8702985656b57764452e854d9c386e9daadc7`; las actualizaciones posteriores del probe y apply constan abajo y en el informe vinculado. Proyecto explícito `espacigo-testprod` (`447373903153`), región `southamerica-west1`, raíz Terraform `infra/terraform/environments/testprod/gcp03-scratch`, state remoto `espacigo/testprod/gcp03-scratch`.

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

## Evidencia de V60: hipótesis local y resultado Cloud SQL

La prueba local aislada usó PostgreSQL `18.6`, un migrador `LOGIN CREATEROLE NOSUPERUSER NOCREATEDB NOINHERIT`, y un contenedor sin red ni volumen. Allí crear el rol GADM y ejecutar inmediatamente el COMMENT funcionó; tras reconciliar a cero membresías, el mismo COMMENT falló con falta de `ADMIN OPTION`. Este es un resultado local, no una regla universal sobre Cloud SQL administrado.

El probe real posterior en Cloud SQL PostgreSQL `18.6` dio un resultado distinto. `role_comment_prepare` y `role_comment_after_reconcile`, cada uno una vez, verificaron el login no superusuario con `CREATEROLE`, atributos restringidos de `espacigo_gadm_bootstrap`, cero membresías entrantes/salientes y ejecutaron exitosamente el comentario literal, con SQLSTATE vacío. La operación Cloud SQL Admin `users.update` conservó solo `cloudsqlsuperuser`. Evidencia y límites: [resultado administrado](gcp03_role_probe_execution_20261016.md). El bloqueo estrecho del COMMENT queda despejado empíricamente para este actor/configuración; no se debe seguir describiendo como imposible.

El probe no ejecutó V60 ni V1–V61. La migración completa, su bloque de creación/reutilización del rol, grants por columna/tabla, trigger, ownership, ACL y runtime siguen sin probarse en Cloud SQL. La base scratch requiere un ensayo controlado con el flujo original de replay V1–V61; el ADR de baseline migrado sigue `PROPOSED` y no hace falta aceptarlo ahora. El alcance detallado está en [replay Cloud SQL V1–V61](gcp03_cloudsql_v1_v61_replay_scope_20261016.md).

## Próximo gate: replay completo, pendiente de planes y revisión

El flujo propuesto sigue siendo `bootstrap` → reconcile de membresías por Cloud SQL Admin → `database_acl` con credencial owner separada → `migrate` V1–V61 → `finalize` → `retire`. La reconciliación se coloca después de bootstrap y antes de migrate. El grupo GADM ya existente se valida/reutiliza sin recrearlo; la versión PostGIS real debe capturarse en la fase bootstrap y no asumirse desde una matriz documentada.

El próximo gate usa el plan local no aplicado de [IAM temporal de credenciales](gcp03_replay_credential_iam_plan_20261016.md). El orden es: revisar permisos/procedimiento y hash → liberar/aplicar el plan IAM exacto → liberar creación CLI de las dos credenciales y registrar sus versiones devueltas → preparar/revisar el plan Jobs con esos números exactos → liberar apply y ejecuciones fase por fase. Las versiones no se inventan antes de crearse; el plan IAM no espera que existan.

Antes de crear nuevas credenciales o Jobs se requieren preflight del scratch, estado vacío/historial ausente y planes Terraform exactos aprobados. La contraseña owner `postgres` y su secreto serán distintos del login/secret migrador; cada fase recibe solo su secreto fijo. Consultar [el alcance de replay](gcp03_cloudsql_v1_v61_replay_scope_20261016.md). No cambiar V1–V61, no importar datos y no tocar la base persistente.

## Evidencia y límites

- `terraform plan -refresh=true` se ejecutó contra el backend remoto de este root; solo se conservaron archivos privados de plan/state/logs.
- La OCI se cargó desde el archivo existente, se etiquetó como `source-bbea4e5` y se verificó por digest remoto; no se reconstruyó.
- Se verificó el probe local de permisos y que su contenedor se retiró al terminar.
- El plan Terraform/OCI de la preparación original no se aplicó en ese corte. Posteriormente sí se aplicaron el plan de probe #352 y el stage 1 de protección Jobs #353; ambos constan en [resultado GCP-03](gcp03_role_probe_execution_20261016.md).
- Las únicas sentencias SQL administradas posteriores fueron el CREATE ROLE fijo y dos COMMENT fijos del probe; no se ejecutó V60 ni V1–V61. La base persistente no se usó.
- La versión PostGIS real del scratch todavía no está registrada; no inferirla de la matriz de versiones ofrecidas. El cleanup stage 2 exacto quedó aplicado tras PR #354; detalle y verificaciones posteriores en [resultado GCP-03](gcp03_role_probe_execution_20261016.md) y [gate de cleanup](gcp03_role_probe_cleanup_plan_20261016.md).
