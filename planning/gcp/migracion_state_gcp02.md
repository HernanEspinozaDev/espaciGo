# Procedimiento futuro para migrar el state bootstrap GCP-02

**Estado: preparado, pendiente de PR revisado/fusionado y liberación explícita de la operación. No se ejecutó `terraform init` contra GCS, `terraform init -migrate-state`, ni se escribió ningún objeto.** El state de bootstrap sigue local y `inventory-apis` permanece separado e intacto.

## Destino comprobado

- Proyecto: `espacigo-testprod` (`447373903153`).
- Bucket: `espacigo-testprod-tfstate-447373903153-bootstrap` (ya creado, versioning activo).
- Prefijo GCS: `espacigo/testprod/bootstrap`.
- Workspace `default`: `espacigo/testprod/bootstrap/default.tfstate`.
- Backend config con esos valores: `.local/planning/gcp/bootstrap-remote-state-prep-20261016/bootstrap.backend.hcl`, permisos `0600`.
- El preflight de solo lectura comprobó que el prefijo estaba vacío y `storage.buckets.get`, `storage.objects.get/create/delete/list` para la identidad activa. No escribió objetos ni creó grants. Esa identidad conserva el rol Owner a nivel de proyecto; este resultado **no** demuestra aislamiento IAM entre buckets.

## Secuencia para una futura operación liberada

1. Revisar/fusionar este PR. Antes de migrar, obtener liberación explícita para la operación concreta. Confirmar proyecto, identidad, versión Terraform y que el objeto destino continúa ausente; repetir `buckets.testIamPermissions` solo lectura y comprobar que el state de `inventory-apis` no cambió.
2. Con `umask 077`, copiar el state local bootstrap a un backup privado nuevo bajo `.local/` (directorio `0700`, archivo `0600`). Verificar SHA-256, serial, lineage y conteo de recursos frente al manifiesto privado de esta preparación. Mantener el backup anterior.
3. Conservar exactamente el `TF_DATA_DIR` que contiene la metadata del backend local de esta raíz: `.local/planning/gcp/bootstrap-state-plan-20261016/tfdata`. Esa metadata apunta al state local privado de bootstrap. No usar `-reconfigure` ni eliminar metadata para olvidar el origen. Confirmar `git status` limpio y el HEAD revisado.
4. En una ventana controlada, desde `infra/terraform/environments/testprod/bootstrap`, con el backend GCS del código revisado y backend config privado ya preparado, ejecutar únicamente:

   ```sh
   umask 077
   export TF_DATA_DIR="$PWD/.local/planning/gcp/bootstrap-state-plan-20261016/tfdata"
   terraform init -input=false -backend-config="$PWD/.local/planning/gcp/bootstrap-remote-state-prep-20261016/bootstrap.backend.hcl" -migrate-state
   ```

   Revisar cualquier resumen/confirmación que muestre Terraform y cancelar si el origen/destino, conteo o workspace no coincide. No usar `-force-copy`, `-reconfigure`, ni otra raíz/state en esta operación.
5. Verificar con `terraform state list` que permanece exactamente el state bootstrap esperado; leer/exportar de forma privada el state remoto y comparar recursos, serial, lineage y contenido lógico con el backup local. Confirmar que el único objeto creado es el destino indicado y que no apareció state bajo otro prefijo.
6. Ejecutar un plan autenticado con refresh desde la raíz y configuración backend aprobadas. Exigir 0 add, 0 change, 0 destroy. Si difiere, detenerse; no aplicar ni “arreglar” en esta migración.
7. Conservar el backup local `0600` hasta revisión/restauración aprobada. Definir y revisar retención de generaciones antes de operación sostenida. El versionado del bucket protege generaciones, pero no reemplaza una copia de recuperación probada.

## Rollback y límites

Si `init` falla antes de copiar, no modificar ni eliminar el state local; diagnosticar. Si la copia parece completar pero la verificación remota falla, detener operaciones Terraform, conservar ambos estados/metadata, descargar de manera privada lo que exista y pedir revisión; no borrar el objeto remoto ni volver a ejecutar init con flags de reconfiguración. El rollback debe ser una operación separada, revisada y basada en el backup, evitando escritores concurrentes.

No migrar `inventory-apis`, GCP ni Cloudflare en esta operación. No crear IAM, habilitar APIs, activar facturación, aplicar Foundation, SQL, Run o DNS. La configuración remota propuesta cambia el destino futuro del backend bootstrap; la migración real queda bloqueada hasta la liberación específica.
