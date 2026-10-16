# GCP-03: apply IAM de credenciales y plan de Jobs — 2026-10-16

## Alcance liberado y resultado

PR #355 fue aprobado y fusionado por `HernanEspinozaDev` en `f4b6b1ecc32e7fc823317ef49a0682948b9639ca`. La revisión liberó aplicar solo el binario IAM exacto y crear las dos credenciales fijas del scratch. El operador GCP activo fue `pasteleriahijitos@gmail.com`; la instancia fue `espacigo-gcp03-pilot01-pg`, base `espacigo_scratch`, proyecto `espacigo-testprod`.

Antes del apply comprobé el SHA del binario y JSON aprobados, la igualdad byte a byte entre `terraform show -json` y su JSON guardado, el snapshot del state y que el vencimiento absoluto aún no hubiera ocurrido. El state conservaba lineage `11eb96d4-a301-9029-11d1-69e1a00f6302`, serial 11 y las 15 direcciones del snapshot del plan; los atributos coincidían salvo tres campos `etag` calculados por Google en recursos IAM. La identidad GCP coincidía con el operador del plan.

Plan IAM aplicado a las `2026-10-16T14:34:21Z`:

- Binario aprobado/aplicado: SHA-256 `837464e26a6cc767e9d1e0a076809b2f87d39e3060aec846b56201e73b40d351`.
- JSON: SHA-256 `f52b5897c115654f5a9e1ea1223aa2b18171e0a917ab3fa524746157cb094603`.
- Resultado: **5 altas, 0 cambios, 0 destrucciones**.
- `terraform plan -refresh=true` después del apply: **sin cambios**. State serial 12, 17 recursos administrados.
- Verificación directa de APIs: dos roles custom con los permisos del plan, un binding Cloud SQL de proyecto y dos bindings de secretos exactos; operador y expresión temporal `request.time < timestamp('2026-10-16T16:00:00Z')` confirmados. El rol de secretos no incluye `secretmanager.versions.access`.

## Credenciales scratch

Con el CLI `cmd/gcp03scratch-credential` fijado al proyecto, instancia, login y nombres de secretos de GCP-03, y sin pasar contraseñas en argumentos:

- A las `2026-10-16T14:35:39Z`, se creó `espacigo_scratch_migrator`; Secret Manager creó `projects/447373903153/secrets/espacigo-gcp03-pilot01-migrator-password/versions/4`.
- A las `2026-10-16T14:35:59Z`, se rotó la contraseña del login `postgres` ya existente; Secret Manager creó `projects/447373903153/secrets/espacigo-gcp03-pilot01-db-owner-password/versions/1`.
- Lectura de metadata confirmó ambas versiones `ENABLED` y la presencia de los dos logins. No se leyó ningún payload.
- El CLI genera 32 bytes aleatorios en memoria, los usa para las operaciones Cloud SQL/Secret Manager y solo informa la referencia de versión. Logs, referencias y verificaciones permanecen bajo `.local/testing/gcp03-replay-credential-iam-plan-20261016/`, excluido de Git, directorio 0700 y archivos 0600. No se guardó ningún valor de contraseña.

## Plan de Jobs preparado, no aplicado

Se generó contra el state posterior, manteniendo `enable_replay_credential_permissions=true` mientras `enable_job=true`, con el digest OCI revisado y versiones numéricas activas:

- Digest: `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`.
- Mapeo: `bootstrap`, `migrate`, `finalize` → service account/secreto migrador, versión `4`; `database_acl`, `retire` → service account/secreto owner, versión `1`.
- Delta machine-checked: **12 altas, 0 cambios, 0 destrucciones**: cinco Jobs fijos, cinco bindings `roles/run.invoker` y dos `roles/iam.serviceAccountUser`. Los grants temporales IAM ya aplicados son `no-op`; no hay cambios no relacionados.
- Binary plan `.local/testing/gcp03-replay-credential-iam-plan-20261016/replay-five-jobs.tfplan`, SHA-256 `bc1a4241622027a3981cf2068ef832b0339f97628cc501b0ea382fa6edc786c7`.
- JSON privado `jobs-plan.json`, SHA-256 `9f1bf8338b179be0aabd46592042c5351de2ec0f5da1c530ab6469558919935e`.
- Hora de generación: `2026-10-16T14:36:35Z`. Las condiciones IAM asociadas vencen a `2026-10-16T16:00:00Z`; no se extendieron. Si no se revisa/libera dentro de una ventana operativamente suficiente, no aplicar el plan ni ampliar el TTL en silencio.
- Una lectura posterior de Cloud Run API confirmó que ninguno de los cinco Jobs de replay existe tras la planificación.

Este plan no fue aplicado. No hay Jobs de migración creados ni ejecutados, ni se ejecutó SQL, V1–V61, bootstrap, ACL, importación o restore. La base persistente y el dataset no se tocaron.

## Compatibilidad aún pendiente

No declarar compatibilidad completa con PostgreSQL/PostGIS por los 16 booleanos actuales. Antes de cerrar esa afirmación, la observación fija pendiente debe registrar `server_version_num`, versiones reales de `pg_extension` y `postgis_full_version()`, ownership de relaciones/secuencias/funciones, atributos/owners/`proconfig`/`search_path`/ACL de funciones `SECURITY DEFINER`, ACL efectivas y consultas espaciales representativas con geometrías fijas. El OCI actual no emite todas esas observaciones. La matriz y el límite de evidencia constan en [alcance del replay V1–V61](gcp03_cloudsql_v1_v61_replay_scope_20261016.md); esas lecturas no se ejecutaron en este paso.

## Siguiente gate

Revisión/liberación exacta del plan Jobs, preflight de state, IAM vigente, digest y versiones, y confirmación de tiempo suficiente antes de la expiración. Si se libera, aplicar el binary plan exacto y ejecutar cada fase por separado, deteniéndose ante cualquier fallo o estado inesperado. El siguiente plan no autoriza las ejecuciones por sí solo; cada paso conserva su propio preflight y evidencia.
