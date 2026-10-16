# GCP-03 verify-retired: artefacto y plan revisable

## Alcance y origen

Esta evidencia continúa la liberación aprobada de GCP-03 después de fusionar PR #361 (`a8fe2714c25c24defd5d7fe0e479aece0fb255bf`). La imagen OCI se construyó desde ese `main` limpio, que incluye la corrección revisada del transporte de `verify_retired`. No contiene credenciales ni el plan Terraform.

- Plataforma: `linux/amd64`; Go `1.27.1`; builder fijado en `golang:1.27.1-alpine3.24@sha256:cd9a32216aee5667f957a62d13a10032a63fd58e14b3f3d9cc8c2122f501e95e`.
- Buildx `v0.38.0`; Docker `29.9.1`.
- Digest OCI local: `sha256:99355889a1cbcf209b7b5dc3906b62780eb1d0f429129df1d14e59667aa98935`.
- SHA-256 del archivo OCI local: `ac2e33f96eff9a9ac54d41bd8d47248ecc58becc8c3f13e4eb2a8046d2c9c743`.
- Publicado exclusivamente en el Artifact Registry scratch ya existente: `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate:verify-retired-a8fe271`.
- Lectura posterior del digest remoto y `docker pull` devolvieron el mismo digest OCI: `sha256:99355889a1cbcf209b7b5dc3906b62780eb1d0f429129df1d14e59667aa98935`.

El manifiesto completo y no sensible está versionado en [gcp03_verify_retired_manifest_20261016.json](gcp03_verify_retired_manifest_20261016.json). Confirma 61 migraciones V1–V61 y el inventario histórico `505f24b385ec8a98a6d27b0367178476a940bce4e45c8a5305fc0a64de72c4a9`. El inventario de los cuatro SQL de bootstrap (`001_pre_migrate.sql` a `004_grant_owner_history.sql`) es `356f10069cbce6cdbb909e5cc45515c91bc14003cf644187118a2c7fbd2fd85a`, coincidente con la imagen de grant-owner-history previamente publicada. Estos hashes verifican los archivos empaquetados; no son una consulta al historial de la base.

## Inventario previo y destino del plan

Lecturas de solo metadatos confirmaron el proyecto `espacigo-testprod`, región `southamerica-west1`, repositorio scratch `gcp03-scratch-pilot01`, instancia `espacigo-gcp03-pilot01-pg` y base `espacigo_scratch`. Ya existen los siete Jobs previos, el service account `gcp03-scratch-acl-pilot01`, su binding `roles/iam.serviceAccountUser` para `pasteleriahijitos@gmail.com`, y los invokers de los Jobs existentes revisados. La versión numérica `1` de `espacigo-gcp03-pilot01-db-owner-password` figura `ENABLED`; no se leyó su payload.

El plan preserva esas identidades y grants. El nuevo Job todavía no tiene invoker, como corresponde: su binding es una de las dos altas planificadas. No falta `actAs` para el service account ACL. Las expiraciones de IAM registradas a `2026-10-16T18:00:00Z` ya pasaron al preparar el plan; el input conserva exactamente ese instante. El plan no las renueva ni cambia esos bindings.

## Plan exacto

- Terraform `1.16.6`, provider Google `8.7.0`; backend GCS del root aislado `espacigo/testprod/gcp03-scratch`.
- Resultado refrescado: **2 to add, 0 to change, 0 to destroy**. Verificación por `terraform show -json`: las únicas acciones no-op excluidas son crear `google_cloud_run_v2_job.retired_verification[0]` y `google_cloud_run_v2_job_iam_member.retired_verification_operator[0]`; todos los demás recursos gestionados quedan no-op.
- El Job queda fijado al digest anterior y al destino exclusivo `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`.
- Identidad: `gcp03-scratch-acl-pilot01@espacigo-testprod.iam.gserviceaccount.com`; operador exacto `user:pasteleriahijitos@gmail.com`.
- Fase `verify_retired`, conexión Cloud SQL privada, una tarea, paralelismo 1, cero reintentos, timeout 1800 s, protección contra borrado y secreto owner versión numérica `1`.
- Hash SHA-256 del plan binario privado: `24940298f6c7804b447004973a1f833ba1c3bc07a7eb2f15ccd7130c69f83f40`.
- Hash SHA-256 del JSON privado mostrado desde el plan: `06a1013015594d491dce3d86fb722141161c9a5d207e5f28607a43e1ee62f503`.

Los archivos binario, JSON, variables y logs completos se conservan en `.local/testing/gcp03-verify-retired-20261016/` con permisos `0600` y directorio `0700`; no se agregan al repositorio. El resumen anterior es la representación revisable del plan, y sus hashes permiten reconocer los archivos locales exactos.

## Verificaciones y límites

`terraform validate` pasó; `terraform test -no-color` pasó con 27/27; `terraform fmt -check` pasó. Se refrescó el plan y se comprobó por máquina el digest, target, SA, secreto/version, fase, número de tareas/reintentos y que solo haya dos altas.

Esta preparación no aplicó Terraform, no ejecutó ningún Cloud Run Job y no envió SQL a Cloud SQL. No leyó payloads de secretos ni verificó las filas/checksums existentes en la base. La comprobación de V1–V61 corresponde al inventario local empaquetado; el informe de `verify_retired` en Cloud SQL sigue pendiente de una revisión del plan/digest y de una ejecución autorizada posterior. No se renovaron IAM temporales ni se repitió `grant_owner_history`, `retire`, revocación o login.
