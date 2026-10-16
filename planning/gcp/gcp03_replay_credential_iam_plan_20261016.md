# GCP-03 V1–V61 replay credential IAM plan — 2026-10-16

## Estado y alcance

El plan IAM exacto fue revisado en PR #355 y aplicado el 2026-10-16 a las 14:34:21 UTC. Resultado: 5 altas, 0 cambios, 0 destrucciones. Luego se creó el login migrador scratch y su versión 4, y se rotó la contraseña del login PostgreSQL `postgres` con versión owner 1. No se aplicó el plan de Jobs, no se ejecutó SQL ni migración y no se autoriza todavía el replay. Evidencia: [apply IAM y plan de Jobs](evidencia_apply_iam_credential_and_jobs_plan_20261016.md).

El gate de permisos puede permanecer habilitado junto al gate separado de Jobs para conservar estos grants durante el plan posterior; habilitar Jobs no activa IAM automáticamente. Las condiciones IAM expiran a las `2026-10-16T16:00:00Z`; no renovarlas sin una revisión y liberación nuevas.

Target explícito: proyecto `espacigo-testprod`, raíz `infra/terraform/environments/testprod/gcp03-scratch`, workspace `default`, base/instancia existentes `espacigo_scratch` / `espacigo-gcp03-pilot01-pg`. `allow_scratch_destroy=false`. Se mantiene separado de los role-probe gates y utiliza IDs custom nuevos, sin reutilizar los IDs soft-deleted `gcp03roleprobe_*`.

## Cambios exactos del plan

Resultado Terraform: **5 crear, 0 cambiar, 0 destruir**. Son los únicos cambios no-op:

1. Rol custom de Cloud SQL `gcp03v1v61sqlops261016` con `cloudsql.instances.get`, `cloudsql.users.create`, `cloudsql.users.get` y `cloudsql.users.update`. No incluye `cloudsql.users.delete`.
2. Binding temporal de ese rol al operador explícito `user:pasteleriahijitos@gmail.com`, con expiración absoluta `2026-10-16T16:00:00Z`.
3. Rol custom nuevo `gcp03v1v61secretversions261016` con `secretmanager.versions.add/destroy/get/list`. Excluye `secretmanager.versions.access`, por lo que no permite leer payloads.
4. Binding temporal del rol de versiones al operador únicamente sobre `espacigo-gcp03-pilot01-migrator-password`.
5. Binding temporal del mismo rol únicamente sobre `espacigo-gcp03-pilot01-db-owner-password`.

Los permisos Cloud SQL `users.*` son de alcance de proyecto según la limitación de Cloud SQL Admin API; el grant es temporal y la operación CLI valida nombres de proyecto, instancia y logins fijos. Los grants de Secret Manager son por recurso secreto individual. Ninguno de estos permisos se concede a las service accounts runtime. No hay `Owner`, `Editor`, `Cloud SQL Admin`, `Secret Manager Admin`, Job, secret resource/version, usuario SQL, base, instancia, red o importación en el delta.

## Artefactos privados y revisión

- Imagen prevista para el replay separado: `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`; source commit `ddd01c6b7ac5437a70453bc52ad17deaa3c96238`; inventario V1–V61 `505f24b385ec8a98a6d27b0367178476a940bce4e45c8a5305fc0a64de72c4a9`. Este plan IAM no crea ni ejecuta Jobs.
- Terraform `1.16.6`, provider Google `8.7.0`; `terraform validate` pasó. `terraform plan` leyó el state remoto y produjo el delta exacto anterior (exit 2 esperado por cambios).
- Binario `.local/testing/gcp03-replay-credential-iam-plan-20261016/replay-credential-iam.tfplan`, SHA-256 `837464e26a6cc767e9d1e0a076809b2f87d39e3060aec846b56201e73b40d351`.
- JSON privado `.local/testing/gcp03-replay-credential-iam-plan-20261016/plan.json`, SHA-256 `f52b5897c115654f5a9e1ea1223aa2b18171e0a917ab3fa524746157cb094603`.
- Plan, JSON, vars y logs son locales/excluidos y modo 0600 en directorio 0700. Inspección machine-readable confirmó las cinco direcciones, permisos exactos, tres condiciones de expiración y dos secretos; no hay Jobs ni recursos de credenciales/SQL.
- La regresión de Terraform incluye un `apply` exclusivamente con `mock_provider` para sembrar el state IAM y el plan secuencial de los cinco Jobs contra ese mismo state aislado. La prueba confirma que IAM, expiración y targets permanecen presentes. También verifica gates apagados por defecto, compatibilidad de ambos gates, mapeo de cinco fases a SA/secreto/versión y rechazo de `latest` y `0`.
- `terraform fmt -check`, `terraform validate` y `terraform test -no-color` pasaron; **20 pruebas** aprobadas, 0 fallidas. El apply secuencial pertenece únicamente al state en memoria de Terraform test con proveedor mock; no contacta GCP.
- El apply del plan revisado se registró en [evidencia apply IAM y plan Jobs](evidencia_apply_iam_credential_and_jobs_plan_20261016.md). Sus artefactos privados de apply, logs y recibos están en `.local/testing/gcp03-replay-credential-iam-plan-20261016/`, modo 0600, directorio 0700.

El expiry es absoluto. El binario IAM ya fue aplicado y no debe repetirse. Los bindings creados expiran a las `2026-10-16T16:00:00Z`; al vencer, un plan o ejecución posterior requiere revalidar IAM y una nueva revisión/liberación. No renovar silenciosamente.

## Secuencia y siguiente gate

1. El apply IAM y las dos operaciones CLI de credenciales están completos según la evidencia enlazada. Los recibos retienen solo las referencias numéricas de versión, nunca payloads.
2. El plan Jobs fue preparado con `enable_replay_credential_permissions=true`, `enable_job=true`, la OCI fijada y las versiones migrador `4`/owner `1`. Contiene 12 altas exactas y no se aplicó. Binario SHA-256 `bc1a4241622027a3981cf2068ef832b0339f97628cc501b0ea382fa6edc786c7`; JSON `9f1bf8338b179be0aabd46592042c5351de2ec0f5da1c530ab6469558919935e`.
3. Aplicar los Jobs requiere revisión/liberación exacta de ese binario, preflight sin drift, digest y versiones activas, además de tiempo suficiente antes de la expiración IAM. Tras la expiración no continuar con el mismo alcance de permisos; revalidar y solicitar una revisión/liberación nueva. El OCI incluye siete fases; seleccionar solamente `bootstrap`, `database_acl`, `migrate`, `finalize` y `retire`.
4. No usar la demo local, importar un dataset ni restaurar datos.

