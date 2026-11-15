# Preparación del plan de espacios para main

## Alcance

Petición humana: preparar todo el trabajo de `plan_creacion_espacios_galeria_documentos.md`, sincronizando código y migraciones para su futura integración a main. Este recibo registra preparación, no publicación, aceptación de producto ni aplicación de SQL.

## Candidato aislado

- Worktree: `/home/nandev/Documentos/projects/espaciGo-spaces-main-preparation`.
- Rama: `codex/marketplace/chore/spaces-main-preparation`.
- Base HEAD: `647cd491985d619370990d3f226b5df61e36b42b`.
- Base remota observada: `origin/main` = `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`.
- Se comprobó que origin/main es ancestro del HEAD: el candidato conserva main y los commits previos de recuperación de borrador, sin reemplazar archivos recientes con una copia antigua.
- Se copiaron 91 archivos modificados/nuevos del worktree de recuperación, verificando igualdad SHA-256 para cada copia. El manifiesto `espacios-main-preparation-manifest.json` conserva las rutas y digests de importación; no es un digest del candidato final después de correcciones.
- Se preservaron los otros worktrees, su estado y sus cambios privados. No se copiaron `.local`, credenciales, volúmenes, datos, node_modules ni state Terraform.
- Hook local configurado en `.githooks`. No se hizo commit, push ni integración a main.

## Sincronización de migraciones

El inventario de archivos SQL del candidato contiene **84 migraciones**, todas byte a byte idénticas a origin/main según comparación programática. No hay delta SQL nuevo de este candidato que deba inventarse o extraerse de una base local. El manifiesto registra nombre, versión numérica y SHA-256 de cada archivo.

`db/release-policy.json` ya registra V84 Didit con checksum exacto y estrategia expand. Esto acredita registro en fuente, no aplicación remota ni autoridad DDL. Las migraciones de espacios previamente integradas se conservan intactas; V84 pertenece a identidad Didit y no debe renombrarse como una migración de galería ni omitirse para forzar el release.

Destino cloud de la entrega: proyecto `espacigo-testprod`, región `southamerica-west1`, instancia `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`. No se exporta ni sustituye esa base con datos locales.

Estado de Cloud SQL no leído nuevamente en esta preparación. El recibo de inspección en el worktree `espaciGo-db-recovery-readonly`, `planning/gcp/evidence_recovery_storage_operational_20261107.json`, documenta observación del 2026-11-08: motor18.6, ledger V1–V83, V84 pendiente y nombres/checksums coincidentes. El padre terminó FAILURE por comprobación de limpieza; el recibo separado verificó ausencia del Job y retiro IAM. Es evidencia histórica, no preflight fresco ni permiso para recrear IAM.

El lock del release fallido y el error V84 `42501 permission denied for schema public` se conservan como bloqueo: sincronización de código/archivos SQL no equivale a sincronización de la base. Debe verificarse ledger, catálogo y autoridad mediante el procedimiento mantenido antes de DDL; no otorgar CREATE, transferir owners, eliminar locks ni reejecutar aplicación por inferencia.

## Revisión de integración: preparación reunida con bloqueos explícitos

- Frontend/contrato y diferencias respecto de main.
- Backend, migraciones y separación de trabajo de otros worktrees.
- Terraform/runtime/configuración del release de media: corregida la duplicación de recursos media. Ambos gates usan las direcciones ya aplicadas `profile_space_media_only[0]` y `api_profile_space_media_only[0]`, exigiendo proyecto/número/región/sufijo revisados. Se mantienen UBLA/PAP/versionado/prevent_destroy, sin moved/import ni apply. Padre revisó el contenido y ejecutó `terraform fmt -check` 1.16.7, código 0; no se ejecutó plan ni se acredita resultado remoto de esta corrección.
- Incompatibilidad de PlanResult corregida en el candidato: `migration_release_manifest.py` acepta únicamente seis campos legacy u ocho con identidad migradora exacta; `release_jobs.py` reconoce las claves base en logs y entrega el reporte al validador cerrado, que rechaza campos extra/identidad parcial o discordante. El padre inspeccionó el diff y ejecutó `py_compile` de ambos archivos y `git diff --check`, código 0. No se ejecutaron sus pruebas ni un Job real; compatibilidad funcional no acreditada.

## Verificación y límites

No se ejecutaron pruebas por instrucción del usuario. Comprobaciones de integridad de copia, ascendencia Git y checksum SQL sí ejecutadas. No se inició API, navegador, Job, build, backup ni operación SQL. No hay afirmación de producto integrado, persistencia, carga GCS ni despliegue.

La preparación no habilita publicación mientras existan incompatibilidades o gates del release pendientes. La integración/publicación debe seguir `.agents/skills/espacigo-release-flow/SKILL.md`, `planning/gcp/cloudsql_migraciones_incrementales.md` y `planning/flujo_entornos/operacion_main_sin_bastion.md` sin recrear bastión ni saltar controles.

Las revisiones delegadas y las correcciones del padre se adjudican en `espacios-main-review-adjudication.md`. El candidato queda reunido para continuar revisión/integración, no marcado como aprobado ni probado. La observación de carrera de recuperación de borrador y la falta de preflight Cloud SQL siguen abiertas. La base persistente local no se tocó.
