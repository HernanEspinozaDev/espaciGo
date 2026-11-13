# Sincronización del despliegue del flujo de imágenes

## Encargo y límites

El usuario solicitó sincronizar despliegue, Cloud SQL, bucket, API y frontend del flujo de imágenes, trabajando directamente en main. Se conserva la instrucción de no pruebas previas a publicación ni QA online por el agente. No se toca la base local persistente.

Autorización específica existente: lectura temporal `storage.objects.get` exclusivamente del lock y objetos exactos de la inspección, revisada y retirada con readback. No se infiere autorización para modificar permisos SQL, roles ni desbloquear; estas acciones siguen fuera de la operación actual hasta decisión explícita correspondiente.

## Observación control-plane del 2026-11-13

- Main publicado y releído: `2ce1526b375d8184ae5717ee266ddf491935ffd0`.
- GitHub Actions completados en SUCCESS para ese SHA: Application quality `43513766129`, Database migration quality `43513766193`, Frontend architecture and quality `43513766173`. Son controles automáticos posteriores al push, no pruebas manuales ejecutadas por el agente.
- Build canónico `033ac29a-c139-427c-8b4a-a2d3ce8afe39`: FAILURE. `verify-source-quality` SUCCESS; `release-main` FAILURE, exit1. Log real: `prior release ended with retained lock; inspect its recorded outcome`. No hubo nuevo rollout en ese build.
- Lock releído a `2026-11-13T22:05:53.046437+00:00`: generación `1794600804441535`, SHA-256 `61656c3b7a61d1a0c853302666f118f2530e6146327ccb5ad3d0b3ac50e1a30d`, owner build `2dc002a9-5831-4717-8064-6432b98d3e01`, source `bb10309129adad9d7204b9b54bb2259186a34242`. El contenido corresponde al intento V84 fallido; no se borró ni reemplazó.
- Instancia `espacigo-testprod-db` RUNNABLE, POSTGRES_18, región `southamerica-west1`. Backup automático habilitado, retención7 COUNT, PITRfalse. Metadata no acredita ledger ni compatibilidad SQL actual.
- API aún sirve `espacigo-test-api-b73aded906d974a64b71f8adc` al100%, digest `sha256:06b1487a7f1d31b28d9aa2afbe46f691607539323c260eaa419ee82e56db3bc3`; runtime `espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com`. Tiene BLOB_STORE_BACKENDgcs, CLOUD_STORAGE_BUCKETespacigo-test-files-447373903153-gcp02, prefijog03o-1017; no figura CLOUD_STORAGE_MEDIA_BUCKET.
- Web pública y privada sirven sus revisiones `*-b73aded906d974a64b71f8adc` al100%, digest web `sha256:822eead5ba93e47ddd1b4c9e498e9117126ff9891ea7a889235848a03a60584d`. Ambas apuntan al API privado correcto; IAP de privada figura true.
- Media `espacigo-testprod-media-447373903153` live, SOUTHAMERICA-WEST1, UBLAtrue, PAPenforced, versioningtrue.
- Bucket genérico `espacigo-test-files-447373903153-gcp02` devuelve404. No se restaura ni cambia su rol por inferencia; debe distinguirse de media y valorar sus dependencias de runtime reales.

## Trabajo operativo en curso

Revisión de la ruta de inspección ya versionada para fijar la generación restaurada exacta sin relajar guards, y revisión de dependencias de la API/BFF de media. No se relanza el aplicador, no se usa bastión y no se inventa una ruta SQL desde el portátil. Esta evidencia no declara despliegue ni carga/lectura funcional completados.

## Adaptación de la inspección a main

Se importaron únicamente25 archivos de la fuente revisada `36c58e5242f1d90c61b694b283190c163dc3f03e`: dos YAML dedicados, CLI/inspector Go y sus tests, helpers y tests Python de recuperación. No se integró la rama completa ni sus diferencias ajenas. Se preservó PlanResult de ocho campos y sus consumidores en main. `cmd/dbmigrate/main.go` agrega solamente el modo observador y rechaza combinarlo con modos de escritura antes de conectar.

Los pins de `recovery_inspection.py` y `recovery_parent_proof.py` se actualizaron de generación histórica a la generación restaurada exacta `1794600804441535`, observada en control-plane; permanecen fijos build/source/ejecución y binding de lock. No se convirtió en un selector dinámico ni se omitió la comprobación.

La revisión parental descartó una afirmación errónea de la primera hoja: `recovery_iam_policy.py:15` fija **solo** `storage.objects.get`; `LEGACY_PERMISSIONS` existe para verificar/revocar leases históricos, no para construir grants nuevos. `_resources(..., legacy=False)` contiene exactamente lock y request del nuevo parent. La revisión independiente previa al grant está en curso; no se aplicó IAM todavía.

Revisión independiente `deleg_b56adbb1` aprobó el alcance source-only sin bloqueantes demostrables; no acredita ejecución cloud. Preflight control-plane parental con `inspect_failed_release(Cloud(account=...))` pasó contra la generación restaurada: Job UID `3be5fbda-57b6-412c-a989-0996f922204e`, ejecución UID `3c86092e-476c-4fd4-9377-a57d87acdace`, exactamente una ejecución CONDITION_FAILED/failed1/running0, intent generación `1794600808178779`. No hubo conexión SQL, ejecución nueva ni escritura IAM en ese preflight.

Verificación sin pruebas: gofmt de los seis archivos Go propios, py_compile de18 archivos recovery Python y git diff-check correctos. La importación no ejecutó tests, Jobs ni consultas SQL. Los diagnósticos por archivo de Go y Pyright no sustituyen la compilación del paquete/CI; no se declara compilación global actual.

Readback adicional del bucket media: API objectAdmin presente; ningún miembro allUsers/allAuthenticatedUsers. La existencia/configuración live del bucket no depende de las hipótesis de la auditoría source-only. El gate standalone LOCAL_PUBLIC_CATALOG_TRIAL no es la única activación de catálogo: `main.go:442` acepta también LOCAL_BOOKING_TRIAL=1; no se habilita un flag local aislado en Cloud Run para satisfacer una inferencia incompleta.

## Inspección privada ejecutada y lease retirado

- Fuente main publicada `3b39174fef5b173ef5f5c52d1bbaa8d9ed382371`; remoto coincidente. Los tres CI automáticos posteriores al push terminaron SUCCESS: Application43517109700, Database43517109825, Frontend43517109658. No se ejecutaron pruebas manuales previas.
- Archive git exacto, sin archivos privados: 42,718,208bytes, SHA256 `fa7ec102e92490060931097ebeb2a3feb7c0a5be0c0cfb2959fe6cc57f065670`. GCS source `source/1794608089.564961-252ec13f0e504472bc0ad79deb0e3d49.gz`, generación `1794608090267113`.
- Parent dedicado `b07742c6-9a5d-4eaa-8b95-1ceec6e47241` **SUCCESS**, finalizado `2026-11-13T22:27:49.279388Z`. Imagen child `468e9aea-ed30-4079-84ff-910a18d18346` SUCCESS, provisión child `a840d0d2-2da7-41d3-86ad-0dec3180db61` SUCCESS. No fue el release full ni desplegó API/web.
- Se concedió el permiso temporal autorizado **solo** storage.objects.get al provisioner, limitado por condición al lock y `recovery-inspections/b07742c6-9a5d-4eaa-8b95-1ceec6e47241/job-request.json`. Rol propio `espacigoRecoveryRead202611077b273bef`, expiración de una hora. Helper y lectura parental independiente confirmaron rol/selector exactos.
- Se releyó por generación el informe `gs://espacigo-testprod-cloud-build-447373903153/recovery-inspections/b07742c6-9a5d-4eaa-8b95-1ceec6e47241/inspection-report.json#1794608791725029`, SHA256 `3de7a3db329652a6cace4f21afc4b715901333b7a919695813f717f2114355d4`, y pasó el validador de producción contra inventario main. Transacción repeatable read/read only, database `espacigo_dms_stage_20261020`, motor18.6, login `espacigo_migrator_login`, rol `espacigo_schema_owner`.
- **Estado SQL actual verificado:** 83 aplicadas contiguas con nombres/checksums coincidentes; única pendiente V84 `V000084__didit_identity_verification_schema.sql`, checksum `d8cc68f3ea73c6e351a31bd8f88fd0606e8217c028777d5ebca7b406bb852b8f`. No se aplicó SQL.
- **Autoridad faltante confirmada:** `schema_public_create=false` y `usuario_references_id=false`. Ledger SELECT/INSERTtrue; USAGE de login/ownertrue. Owner de public `pg_database_owner`, owner de usuario `postgres`. V84 requiere crear tablas y FK a usuario(id); no basta liberar el lock ni repetirla con la autoridad actual.
- Finalizada la inspección se revocó el lease por el helper revisado; readback independiente confirmó binding0, rol `deleted=true`, baseline preservado y Job inspector exacto ausente. Lock sigue en generación `1794600804441535`. No se amplió ningún permiso SQL, rol de base ni se desbloqueó.

**Gate identificado al cerrar la inspección:** la autorización de despliegue no revocaba por inferencia la prohibición expresa de permisos SQL/desbloqueo. Se solicitó autorización separada para la operación temporal mínima V84 y la recuperación exacta del lock. Backup, DDL por Job y postcheck permanecían como gates separados; la inspección no significó despliegue completado.

**Autorización posterior recibida:** el usuario autorizó la operación acotada V84 con backup, los dos permisos SQL temporales mínimos y su retirada verificada, Job privado y recuperación de la generación exacta del lock. Después autorizó expresamente roles/secretmanager.secretAccessor solo sobre `espacigo-testprod-cloudsql-postgres-admin-password` para `espacigo-test-migrator@espacigo-testprod.iam.gserviceaccount.com`, con expiración/retirada y versión1 montada solo en el Job, sin descargar/mostrar ni dar acceso a API/builder. Backup `1794610719706` SUCCESSFUL verificado. Preparación, revisiones, desviación de pruebas y gates pendientes se conservan en `../../gcp/v84_recovery_operation.md`. Al escribir este addendum no se había concedido IAM, aplicado SQL ni retirado el lock.
