# GCP-03: verificación de retiro en scratch

Fecha: 2026-10-16. Scratch: `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`.

## Despliegue de la fase de solo lectura

- PR #362 fue aprobado por `HernanEspinozaDev` y fusionado con merge `5419926016b5ae900b1843947bbbb9dff698d9e7`; el OCI se construyó desde el commit revisado `a8fe2714c25c24defd5d7fe0e479aece0fb255bf` y se publicó solo en el Artifact Registry scratch como `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:99355889a1cbcf209b7b5dc3906b62780eb1d0f429129df1d14e59667aa98935`.
- El plan binario autorizado tuvo SHA-256 `24940298f6c7804b447004973a1f833ba1c3bc07a7eb2f15ccd7130c69f83f40`; JSON `06a1013015594d491dce3d86fb722141161c9a5d207e5f28607a43e1ee62f503`. El apply del plan exacto agregó únicamente el Job fijo `verify_retired` y su binding `roles/run.invoker` (2 altas, 0 cambios, 0 bajas). La configuración conserva una tarea, cero reintentos, el owner y su versión de secreto explícita 1. La base y los secretos no se recrearon.
- Se invocó una vez, sin overrides, la ejecución `espacigo-gcp03-pilot01-verify-retired-v9tj6`. Terminó con `Completed=False`, `succeededCount=0`, código 1. No se repitió la ejecución.

## Informe recuperado

Cloud Logging devolvió 25 entradas del execution exacto. La reconstrucción produjo 21 registros de informe: manifiesto inicial y 20 fragmentos, esquema 2, 237.688 bytes y SHA-256 del payload reconstruido `ff557ce6c5b5779ae5ad703cc7b7ed0c704dda972e02d3030a1c84f447251ad2`. `ReadIntegrityNDJSON` reconstruyó el informe tipado correctamente; la colección no se considera incompleta. El resultado fue `passed=false`.

- Historia V1–V61, ACL de auditoría, permisos runtime y comprobaciones espaciales pasaron.
- El migrador ya tenía `NOLOGIN`, `NOINHERIT` y los atributos globales restringidos; permanecía una sola membresía: `cloudsqlsuperuser` → `espacigo_scratch_migrator`, `ADMIN=false`, `INHERIT=true`, `SET=true`.
- El contrato final falló en el retiro y membresías. TEMP y CREATE efectivos del migrador continuaban activos por esa membresía. El informe observó 377 propietarios de relaciones, 1.031 propietarios de funciones y 12 funciones `SECURITY DEFINER`; esos inventarios no son una afirmación de seguridad universal de cada objeto.
- El Job era estrictamente de lectura. No ejecutó `REVOKE`, `ALTER ROLE`, migraciones ni SQL de reparación. No cambió login, ACL, ownership, historial ni secretos.

## Evidencia privada y límites

Las respuestas crudas de Cloud Logging, ejecución, reconstrucciones, hashes, plan binario/JSON y estado previo/posterior se conservan en `.local/testing/gcp03-verify-retired-20261016/`, con permisos 0700/0600. Hashes de archivos reconstruidos: NDJSON `ae92ee746926fe5c089b3d7dd23d7611c909993600b7b6cf5240eef86fe5bf92`; JSON `e678c83bb9c4c3970e4c6dc65f74147ff88996429d8469185f6ad4b4ca474cec`.

La falla condujo al desarrollo de una fase separada `revoke_retired_membership`, apagada por defecto. Su test PostgreSQL 18 local comprueba una ruta con ADMIN OPTION y otra sin ella que revierte intacta; no demuestra que el rol administrado de Cloud SQL conceda autoridad equivalente. Aún no se ha construido/publicado OCI nuevo ni preparado/aplicado plan para esa fase, y no se ejecutó en Cloud SQL. Requiere revisión del código, del OCI y del plan exacto antes de considerar una ejecución.

La integración fixture terminó con PASS en el contenedor desechable `postgis/postgis:18-3.6` fijado por digest, puerto loopback 65439. El role local `cloudsqlsuperuser` recibió TEMP en la base y CREATE en `public`, y la membresía heredable hizo que el migrador tuviera ambos permisos antes de la fase. El owner fixture fue `NOSUPERUSER NOCREATEROLE`: con ADMIN OPTION revocó la membresía y el test verificó TEMP/CREATE verdaderos antes y falsos después; sin ADMIN obtuvo SQLSTATE `42501`, dejó ambos permisos heredados presentes y comparó sin cambios los atributos del migrador, ACL/propiedad de `schema_migrations` y el reporte post-rollback. Contenedores, red y volumen se retiraron; la configuración con credencial fixture se eliminó. El log final quedó en `.local/testing/gcp03-membership-fixture-20261016/test.log` con modo 0600. Esta evidencia valida solo el mecanismo PostgreSQL local, no una capacidad de Cloud SQL.
