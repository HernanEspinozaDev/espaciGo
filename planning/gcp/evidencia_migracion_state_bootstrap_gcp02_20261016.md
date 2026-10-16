# Evidencia de migración del state bootstrap GCP-02

**Ejecutado el 2026-10-16 (UTC), tras la fusión y revisión de PR #331.** Este registro documenta únicamente el state bootstrap de los tres buckets; no incluye la migración de `inventory-apis` ni de otro state.

## Preflight y operación

- Proyecto explícito `espacigo-testprod` (`447373903153`), bucket `espacigo-testprod-tfstate-447373903153-bootstrap`, prefijo `espacigo/testprod/bootstrap`, workspace `default`.
- Se confirmó que el prefijo remoto estaba vacío, versioning estaba habilitado y la identidad activa de CLI/ADC coincidía con la registrada. `buckets.testIamPermissions` confirmó `storage.buckets.get` y `storage.objects.get/create/delete/list`. No se modificó la configuración global de proyecto de gcloud.
- La metadata local existente se validó como backend `local` y su ruta coincidió con el state privado esperado. El backup del state original y una copia de la metadata previa se conservaron en `.local/planning/gcp/bootstrap-remote-state-prep-20261016/`, con permisos `0600` dentro de directorio `0700`. Hash SHA-256, serial, lineage y contenido detallado permanecen privados en `migration-preflight-*.json` y `migration-result.json`.
- Se ejecutó `terraform init -backend-config=<backend privado> -migrate-state` en TTY con el `TF_DATA_DIR` original. Terraform informó que el backend anterior era local y que el destino GCS no contenía state; se confirmó `yes` para esa copia única autorizada. No se usó `-force-copy` ni `-reconfigure`.

## Verificación posterior

- Workspace remoto `default`; direcciones: `google_storage_bucket.terraform_state["bootstrap"]`, `["gcp"]` y `["cloudflare"]` (tres instancias).
- El state remoto conserva el mismo lineage y el mismo contenido lógico de las tres instancias que el backup local. El serial avanzó de 4 a 5 durante la migración.
- El listado actual de objetos en el prefijo muestra únicamente `espacigo/testprod/bootstrap/default.tfstate`; no hay lock activo. Un plan autenticado con refresh terminó con 0 altas, 0 cambios y 0 destrucciones (tres recursos `no-op`). No se aplicó ningún plan.
- El bucket tiene versioning: el listado de generaciones muestra tres generaciones no vigentes de `default.tflock` creadas y borradas por las operaciones Terraform, además de una generación no vigente intermedia de `default.tfstate`. Son versiones históricas, no objetos activos; no se borraron. La única generación vigente es el state final. El inventario completo con generaciones y tiempos está en evidencia privada.

## Límites y conservación

No se crearon recursos GCP, no se habilitaron APIs, no se añadieron grants IAM y no se cambió billing. El state `inventory-apis` y su backup permanecen locales y separados. El backup original del state bootstrap y la metadata local previa se mantienen para recuperación. El acceso probado sigue siendo el de Owner a nivel de proyecto: separar buckets no crea aislamiento IAM frente a esa identidad.

El backend de Foundation GCP se está preparando aparte para un plan privado de solo lectura; esa tarea no forma parte de esta migración ni autoriza aplicar Foundation, SQL, Run, edge, DNS, IAM o versiones/valores de secretos.
