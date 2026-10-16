# GCP-03: alcance propuesto para replay V1–V61 en Cloud SQL scratch

Estado actual: se aplicaron los cinco recursos IAM temporales revisados en PR #355, se crearon migrator v4 y owner v1, y se aplicó el plan exacto de Jobs de PR #356 (12 altas, cero cambios/eliminaciones). El replay aislado se completó hasta `finalize`; `retire` y el observador permanecen pendientes. No hubo importación, restore ni acceso a la base persistente. Ver [evidencia del apply IAM y plan Jobs](evidencia_apply_iam_credential_and_jobs_plan_20261016.md) y [evidencia del replay](evidencia_gcp03_replay_20261016.md). El ADR de baseline migrado permanece `PROPOSED`; el replay no implica aceptación del ADR ni compatibilidad total.

## Evidencia administrada disponible

En `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`, PostgreSQL informó `server_version_num=180006`. En los probes previos al replay, `role_comment_prepare` y `role_comment_after_reconcile` comprobaron que `espacigo_gadm_bootstrap` era `NOLOGIN`, tenía atributos restringidos y no tenía membresías. La ejecución de replay posterior está registrada en [evidencia del replay](evidencia_gcp03_replay_20261016.md); el observador de `postgis_full_version()`, ownership/ACL detallados y consultas espaciales todavía no se ejecutó.

Esto despejó el bloqueo estrecho observado en el probe PostgreSQL local, donde la sentencia falló sin `ADMIN OPTION`, para esta configuración administrada concreta. El probe por sí solo no demostraba replay, grants/ownership de V60, extensiones ni runtime; el resultado posterior del replay está separado abajo. Ver [resultado y límites del probe](gcp03_role_probe_execution_20261016.md) y [ADR propuesto](adr_gcp03_baseline_migrado_v60_20261016.md).

Al momento de aquellos probes, sus dos conexiones comprobaron que no había tablas de aplicación; el probe creó el rol `espacigo_gadm_bootstrap NOLOGIN`. El replay ejecutado después usó esa misma base y rol, sin borrar ni recrear el scratch. Bootstrap registró `postgis=3.6.4`, pero aún no se obtuvo `postgis_full_version()`; no inferir más detalles ni copiar la versión del origen.

## Preflight y planes aplicados

1. Se usó la OCI ya revisada `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`, plataforma `linux/amd64`, source commit `ddd01c6b7ac5437a70453bc52ad17deaa3c96238`, inventario migratorio `505f24b385ec8a98a6d27b0367178476a940bce4e45c8a5305fc0a64de72c4a9`. No se reconstruyó ni sustituyó. El plan definió los cinco Jobs `bootstrap`, `database_acl`, `migrate`, `finalize` y `retire`; se ejecutaron solo los primeros cuatro, no la demo local ni fases `role_comment_*`.
2. Los dos roles custom y tres bindings IAM temporales se aplicaron con expiración `2026-10-16T16:00:00Z`; sus lecturas API confirmaron principal, permisos y secretos exactos. Los grants no alcanzan service accounts runtime ni habilitan Jobs automáticamente.
3. El CLI creó el login migrador fijo (`espacigo_scratch_migrator`, versión 4) y actualizó la contraseña del login existente `postgres` (versión owner 1). Los valores aleatorios solo pasaron por memoria; no se leyeron ni publicaron payloads. El state Terraform quedó sin drift después de los applies IAM y Jobs.
4. El plan de Jobs se aplicó después de preflight: 12 altas, cero cambios/eliminaciones. Hashes binario/JSON: `bc1a4241622027a3981cf2068ef832b0339f97628cc501b0ea382fa6edc786c7` y `9f1bf8338b179be0aabd46592042c5351de2ec0f5da1c530ab6469558919935e`. El refresh plan posterior no encontró drift.
5. Las fases completadas una por una, con preflight/resultado verificado, son: `bootstrap`, reconcile de membresías, `database_acl`, `migrate`, `finalize`. Los execution IDs y límites están en la evidencia de replay. `retire` no se ejecutó; el observador requiere un cambio revisado, digest nuevo y plan separado. No renovar en silencio la expiración IAM ni continuar ante fallo/timeout/estado incierto.

## Secuencia aislada y estados

1. **Bootstrap — completado.** Una ejecución como migrador en el scratch existente; sus logs reportaron versiones `btree_gist`, `postgis` y `unaccent`. El SQL fijo verificó la base objetivo, creó extensiones soportadas si faltaban y roles restringidos; reutilizó el GADM creado por probe.
2. **Reconcile de membresías — completado.** Cloud SQL Admin `UPDATE_USER` concluyó `DONE` y el CLI confirmó solo `cloudsqlsuperuser` para el migrador.
3. **Database ACL — completado.** Una ejecución como `postgres` con owner v1 revocó TEMP de `PUBLIC` y CREATE de `PUBLIC` en `public`, concedió TEMP y CREATE únicamente al migrador y pasó las verificaciones fijas.
4. **Migrate — completado.** Una ejecución como migrador pasó los prechecks roles/ACL y ejecutó V1–V61, sin modificar archivos/checksums ni importar datos. `finalize` verificó historia/checksums completos.
5. **Finalize — completado.** `VerifyAllApplied` validó las 61 migraciones/checksums, aplicó `GrantRuntimePermissions` y las 16 verificaciones runtime pasaron. Eso no prueba owners detallados, todos los grants `EXECUTE`, `postgis_full_version()` ni consultas espaciales; continúan como observabilidad pendiente.
6. **Observabilidad separada, pendiente de revisión/aplicación:** hay una implementación local de una fase fija `observe` en `internal/dbbootstrap/observer.go` y `internal/gcp03scratch/observer.go`. Contiene solo consultas constantes `SELECT`, emite un reporte JSON saneado, y falla si fallan los checks espaciales, ACL o runtime. No se ha compilado en OCI, publicado, agregado a Terraform/Cloud Run ni ejecutado en Cloud SQL. El digest actualmente desplegado `3e5683…` no incluye esta fase. El cambio requiere revisión, publicación de un digest nuevo y un plan separado para el Job fijo antes de cualquier ejecución.

   El alcance del observador implementado incluye servidor e inventario de extensiones (con `postgis_full_version()`), roles/membresías fijos, ACL de base/schema y los 16 checks runtime nombrados, owners de relaciones/secuencias/índices/funciones, `proconfig` y ACL `EXECUTE` por función `SECURITY DEFINER`, además de tres operaciones PostGIS sobre geometrías constantes y el estado/definición del índice GIST sintético esperado. No afirma que el planner utilice el índice ni compara ownership con un conjunto esperado; esa revisión sigue siendo humana.

   | Observación | Evidencia que debe emitir | Cobertura del OCI `3e5683…` actual |
   | --- | --- | --- |
   | Servidor/extensiones | `server_version_num`, versiones de `pg_extension` y `postgis_full_version()` | El probe ya registró PostgreSQL `180006`; bootstrap consulta/expone las tres versiones de extensiones. El digest no emite `postgis_full_version()`. |
   | Ownership | Conteos/lista saneada de owner por relaciones, secuencias y funciones | No cubierto por las 16 comprobaciones runtime. |
   | `SECURITY DEFINER` | Nombres/owners, `proconfig`/`search_path` y `EXECUTE` ACL para cada función con `prosecdef` | No cubierto por las 16 comprobaciones runtime. |
   | Roles/ACL | Atributos y membresías del GADM, ACL de DB/schema y permisos efectivos requeridos | Solo cubre los checks que ya implementa el runner; no inferir un inventario total de ACL. |
   | PostGIS | Resultados booleanos de geometrías fijas para validez, intersección y distancia; metadatos de índice espacial esperado | No cubierto por el OCI actual. No declarar que un planner usó el índice si el scratch sin datos no permite demostrarlo. |

   Las consultas quedan como constantes en el ejecutor; no se aceptan SQL, identificadores ni expresiones desde flags, variables o argumentos. Las pruebas locales del contrato de fase y de las consultas no sustituyen una ejecución real. No afirmar que estas comprobaciones ya ocurrieron por las 16 booleans. El OCI de migración ya ejecutado no se reconstruye ni cambia; hasta una revisión y ejecución separadas, mantener esta evidencia pendiente y no declarar compatibilidad total.
7. **Retire**, ejecución como owner `postgres`: volver a validar historia/runtime, deshabilitar login/atributos del migrador y revocar TEMP/CREATE. Luego Cloud SQL Admin revoca `cloudsqlsuperuser`, espera/confirma roles y destruye solo la versión numérica migrador. Rotar la contraseña owner por el CLI y destruir solo su versión numérica tras confirmar la operación.
8. Tras evidencia completa, preparar planes Terraform separados para retirar Jobs/IAM temporales. Verificar bindings y custom roles; estos son soft-delete, guardar IDs/estado y no reutilizar IDs sin conocer retención/undelete. Conservar instancia/base/secret metadata, VPC y volúmenes del scratch.

## Evidencia de salida requerida

Capturar únicamente valores saneados: HEAD/digest, hashes de planes, IDs/estados de operaciones/ejecuciones, versiones numéricas de secretos (nunca valores), versiones reales de extensiones y `postgis_full_version()` cuando el ejecutor aprobado lo permita, historial/checksums, owners/ACL/grants/funciones y resultados espaciales. No guardar passwords, tokens, payloads de secreto ni logs sin filtrar. El alcance actual es solo replay V1–V61 en el scratch existente, sin import/restore/dataset; producción y la base persistente siguen fuera de alcance.
