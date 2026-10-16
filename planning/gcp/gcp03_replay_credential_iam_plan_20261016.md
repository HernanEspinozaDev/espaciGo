# GCP-03 V1–V61 replay credential IAM plan — 2026-10-16

## Estado y alcance

Plan local preparado para la siguiente revisión de GCP-03. No se aplicó. Su propósito es habilitar, durante una ventana corta, el CLI fijo de credenciales para los dos logins del scratch existente. No crea/rota credenciales ni versiones, no modifica recursos SQL/secrets, no habilita Jobs, no ejecuta SQL y no autoriza replay. El gate de permisos puede permanecer habilitado junto al gate separado de Jobs para conservar estos grants durante el plan posterior; habilitar Jobs no activa IAM automáticamente.

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

El expiry es absoluto y el plan deja de servir para una ejecución tras vencerse; revalidar IAM y preparar un plan nuevo si se aprueba después de `2026-10-16T16:00:00Z`. No renovar silenciosamente. **Este plan no se aplicó**.

## Secuencia obligatoria después de revisar este plan

1. Revisar el plan exacto, el procedimiento CLI y el riesgo project-wide de Cloud SQL. Tras la revisión/liberación exacta y un preflight sin drift, aplicar solo este binario vigente.
2. En una liberación posterior de creación de credenciales, el CLI actual crea/actualiza el login migrador y su versión de secreto, además de rotar `postgres` y crear su versión owner. Registrar únicamente los números devueltos y estado saneado; no anticipar números ni copiar valores. Los secretos/versiones se crean fuera de Terraform.
3. Con ambos números reales activos, generar un plan nuevo para los cinco Jobs manteniendo `enable_replay_credential_permissions=true` y habilitando `enable_job=true`; así el plan conserva los grants ya aplicados. Los Jobs deben fijar `migrator_secret_version` para `bootstrap/migrate/finalize` y `database_owner_secret_version` para `database_acl/retire`, nunca `latest`. Revisar el delta/hashes y solo ejecutar tras su liberación exacta.
4. El OCI incluye las cinco fases fijas de replay y las dos fases fijas del probe; seleccionar solamente `bootstrap`, `database_acl`, `migrate`, `finalize` y `retire`. No usar la demo local, no importar un dataset y no restaurar datos.

