# Recuperación temporal de state y Cloud Build — 2026-11-13

## Autorización y alcance

El usuario pidió restaurar temporalmente los recursos necesarios y sincronizar el almacenamiento de imágenes tras el build. Las restauraciones iniciales de state y Cloud Build y las acciones de media se registran en sus secciones fechadas. Proyecto: `espacigo-testprod`. No se autorizó ni ejecutó ninguna operación PostgreSQL, ni se restauraron buckets de contratos/KYC, ni se publicó a `main`.

## Restauración verificada

- Antes de restaurar, ambos buckets estaban en soft-delete; sus generaciones verificadas fueron `1792114656222205050` (state GCP) y `1792634177265473896` (Cloud Build). La retención reportada terminaba el 2026-11-20.
- `gcloud storage restore` restauró ambos buckets por generación. Para state se restauraron las 154 generaciones de 12 objetos `*.tfstate`; no se restauraron objetos de lock Terraform. El bucket contiene 11,064,458 bytes live. Quedaron objetos/versiones soft-deleted, por lo que no se declara recuperación total de todas las generaciones históricas.
- Se verificó el backend GCS con el procedimiento `scripts/gcp07-verify-state-backend.sh`: workspace `default` disponible. `terraform state list` leyó 80 recursos; entre ellos permanecen registrados `google_storage_bucket.cloud_build[0]`, `google_storage_bucket.private_files[0]` y sus IAM asociados.
- Terraform requerido por esta raíz es `>=1.16.0, <1.17.0`; la versión del sistema es 1.17.2. Para la verificación se descargó temporalmente Terraform 1.16.7 bajo el scratch privado, se comprobó el SHA-256 oficial/local `5454efcf7fc87f0c2113111b9a5c7182f0ca1c6f0f23a1e110a50770a8fe473b`, y se eliminó el directorio temporal del procedimiento de backend.
- Cloud Build quedó live con 165 objetos y 2,050,902,313 bytes. Sus políticas observadas son región `SOUTHAMERICA-WEST1`, UBLA activado, Public Access Prevention `enforced`, retención soft-delete de 7 días y lifecycle de 30 días solo para `source/`. Versionado de objetos no figura habilitado; versiones históricas anteriores permanecen en soft-delete y no se afirma que todas estén live.
- No se restauró el bucket de `run-sources` ni `espacigo-test-files-447373903153-gcp02`.

## Gate de release descubierto; no desbloquear

El lock live `cloudrun-main.lock` pertenece al build `2dc002a9-5831-4717-8064-6432b98d3e01` (source SHA `bb10309129adad9d7204b9b54bb2259186a34242`). El build terminó `FAILURE` el 2026-11-07. Su preflight registró pendiente `V000084__didit_identity_verification_schema.sql`; se creó e inició el Job de aplicación, que terminó con exit code 1. Cloud Logging informó `permission denied for schema public` (SQLSTATE `42501`). No existe evidencia `migration-verified.json` ni `postcheck.json` para ese intento. `scripts/gcp/recover_release_lock.py` está diseñado para rechazar el desbloqueo cuando existe `job-intent-apply.json`.

No se consultó ni modificó PostgreSQL ni se cambiaron grants de PostgreSQL. El lock debe mantenerse hasta revisión autorizada del resultado de V84; no iniciar otro release full ni borrar/reemplazar el lock. No resolver el error elevando privilegios sin una revisión independiente.

## Estado inicial de la sincronización antes de la provisión de media

La configuración leída del servicio `espacigo-test-api` tiene `BLOB_STORE_BACKEND=gcs`, `CLOUD_STORAGE_BUCKET=espacigo-test-files-447373903153-gcp02`, `CLOUD_STORAGE_OBJECT_PREFIX=g03o-1017` y no tiene `CLOUD_STORAGE_MEDIA_BUCKET`. Ese bucket de archivos sigue en soft-delete. El código local de la rama acepta `CLOUD_STORAGE_MEDIA_BUCKET`, pero no se creó el bucket de media ni se configuró/desplegó el servicio. Los buckets de Build almacenan staging/evidencia; las imágenes de usuario se escriben por el API en runtime, no las copia Cloud Build como parte del build de contenedor.

Para completar la carga de fotos hace falta crear y autorizar el bucket privado de media, entregar `CLOUD_STORAGE_MEDIA_BUCKET` al runtime en el release y verificar el flujo según sus gates. El servicio de Cloud Run actual despliega imágenes de Artifact Registry; no consume un bucket personalizado de Cloud Run sources.

## Límites de la primera fase de recuperación

- No se creó ningún bucket con nombre nuevo ni se copiaron objetos a un bucket nuevo.
- No hubo Terraform plan/apply, cambio de IAM o Cloud Run, build nuevo, pruebas, commit, push, merge o despliegue.
- La rama sigue intencionalmente sucia, sin commit; no se integró a `main`.
- Los dos buckets recuperados permanecen live temporalmente mientras se prepara una migración revisada. No se volverán a borrar hasta terminar y verificar cualquier migración autorizada.

## Addendum: provisión de media y preparación del release — 2026-11-13

- Se aplicó un plan Terraform dirigido al bucket `espacigo-testprod-media-447373903153` y a su binding IAM para el API; el state remoto conserva `google_storage_bucket.profile_space_media_only[0]` y `google_storage_bucket_iam_member.api_profile_space_media_only[0]`. La lectura del state devolvió 82 recursos. No se aplicaron los buckets de contratos/KYC.
- Readback de Storage: bucket live en `SOUTHAMERICA-WEST1`, clase `STANDARD`, uniform bucket-level access activado, Public Access Prevention `enforced` y versionado activado. Storage JSON API encontró **0 objetos** en el bucket al consultar; por tanto no se afirma que exista una carga de imágenes ni una prueba de round-trip.
- El IAM directo del bucket contiene `roles/storage.objectAdmin` para `serviceAccount:espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com`; el readback no encontró `allUsers` ni `allAuthenticatedUsers`. Este es un cambio de permiso limitado al bucket de media, no una apertura pública del bucket ni un cambio a las políticas de los servicios web.
- `cmd/api/runtime_storage.go` ya selecciona `CLOUD_STORAGE_MEDIA_BUCKET` cuando está configurado. `scripts/gcp/release_main.py` ahora pasa `CLOUD_STORAGE_MEDIA_BUCKET=espacigo-testprod-media-447373903153` solo al despliegue de la API y verifica el valor en la revisión y en la plantilla del servicio, manteniendo la comparación del resto de configuración. Se ajustó el fake de `scripts/gcp/test_main_release.py`; no se ejecutó esa ni otra prueba.
- Readback actual del servicio `espacigo-test-api`: `CLOUD_STORAGE_MEDIA_BUCKET` sigue ausente. No se desplegó la nueva revisión y no se hizo build nuevo ni carga funcional.
- El build full `2dc002a9-5831-4717-8064-6432b98d3e01` continúa en `FAILURE` al releerlo. El objeto `cloudrun-main.lock` sigue live, generación `1794600804441535`, y pertenece a ese build/SHA `bb10309129adad9d7204b9b54bb2259186a34242`. La evidencia de su Job indica V84 fallida con `permission denied for schema public` (SQLSTATE `42501`), sin postcheck. Diagnóstico solo de código: el Job configura `DATABASE_SET_ROLE=espacigo_schema_owner` (`scripts/gcp/migration_job_plan.py:27`), el runner ejecuta `SET ROLE` (`internal/migrator/runner.go:205-219`) y V84 inicia con `CREATE TABLE` sin esquema explícito (`db/migrations/V000084__didit_identity_verification_schema.sql:3`). Esto es consistente con falta de `CREATE` efectivo sobre `public`, pero no confirma los grants actuales; no se leyó el catálogo ni el ledger. El runner ejecuta SQL y registra ledger en la transacción de migración y llama rollback ante error SQL (`internal/migrator/runner.go:315-329`), pero no se declara el ledger actual verificado. No se tocó PostgreSQL, no se desbloqueó ni se reintentó el release.
- Verificaciones locales de este addendum: `python3 -m py_compile` para los dos scripts Python editados y `git diff --check` terminaron con código 0. No se ejecutaron pruebas de aplicación, build de producto, Cloud Build nuevo ni prueba de carga, conforme a la solicitud de no hacer más pruebas.
- Checkout: rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `origin/main` `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`. El worktree sigue con cambios previos ajenos además de estos cambios; no hubo commit, push, merge ni despliegue.

**Resultado:** bucket e IAM verificados y ruta del release preparada; el runtime aún no está sincronizado. Para activarlo hace falta resolver por el procedimiento autorizado el Job V84 y el lock de release, completar el full release con sus gates y luego verificar el readback del servicio y una carga/lectura real. Ninguno de esos pasos se ejecutó aquí.
