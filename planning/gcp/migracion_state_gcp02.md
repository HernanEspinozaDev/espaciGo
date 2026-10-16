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
3. Capturar la raíz absoluta del repo **antes de cambiar directorio** y conservar exactamente el `TF_DATA_DIR` que contiene la metadata local de esta raíz. Validar que metadata declara `backend.type=local`, que `backend.config.path` resuelve al state local privado esperado y que existen el state y el backend config privado. No usar `-reconfigure` ni eliminar metadata para olvidar el origen. Confirmar `git status` limpio y el HEAD revisado.
4. En una ventana controlada, ejecutar el preflight local y luego el init en terminal interactiva. Las rutas se construyen desde la raíz capturada, no desde el cwd de `bootstrap`:

   ```sh
   set -eu
   repo_root="$(git rev-parse --show-toplevel)"
   bootstrap_root="$repo_root/infra/terraform/environments/testprod/bootstrap"
   tf_data_dir="$repo_root/.local/planning/gcp/bootstrap-state-plan-20261016/tfdata"
   source_state="$repo_root/.local/planning/gcp/bootstrap-state-plan-20261016/bootstrap.tfstate"
   backend_config="$repo_root/.local/planning/gcp/bootstrap-remote-state-prep-20261016/bootstrap.backend.hcl"
   metadata="$tf_data_dir/terraform.tfstate"

   test -d "$tf_data_dir" && test -f "$metadata" && test -f "$source_state" && test -f "$backend_config"
   test "$(stat -c '%a' "$tf_data_dir")" = 700
   test "$(stat -c '%a' "$source_state")" = 600
   jq -e --arg path "$source_state" \
     '.backend.type == "local" and .backend.config.path == $path' \
     "$metadata" >/dev/null
   umask 077
   export TF_DATA_DIR="$tf_data_dir"
   cd "$bootstrap_root"
   terraform init -backend-config="$backend_config" -migrate-state
   ```

   No pasar `-input=false`: Terraform debe poder solicitar la confirmación interactiva de copia local→GCS. Leer el mensaje en la TTY y contestar solo si source, bucket/prefijo, workspace y conteo coinciden con esta revisión; cancelar con Ctrl-C ante cualquier divergencia o si no se puede verificar la pregunta. No automatizar la respuesta ni usar `-force-copy`, `-reconfigure` u otra raíz/state.
5. Verificar con `terraform state list` que permanece exactamente el state bootstrap esperado; leer/exportar de forma privada el state remoto y comparar recursos, serial, lineage y contenido lógico con el backup local. Confirmar que el único objeto creado es el destino indicado y que no apareció state bajo otro prefijo.
6. Ejecutar un plan autenticado con refresh desde la raíz y configuración backend aprobadas. Exigir 0 add, 0 change, 0 destroy. Si difiere, detenerse; no aplicar ni “arreglar” en esta migración.
7. Conservar el backup local `0600` hasta revisión/restauración aprobada. Definir y revisar retención de generaciones antes de operación sostenida. El versionado del bucket protege generaciones, pero no reemplaza una copia de recuperación probada.

## Rollback y límites

Si `init` falla antes de copiar, no modificar ni eliminar el state local; diagnosticar. Si la copia parece completar pero la verificación remota falla, detener operaciones Terraform, conservar ambos estados/metadata, descargar de manera privada lo que exista y pedir revisión; no borrar el objeto remoto ni volver a ejecutar init con flags de reconfiguración. El rollback debe ser una operación separada, revisada y basada en el backup, evitando escritores concurrentes.

No migrar `inventory-apis`, GCP ni Cloudflare en esta operación. No crear IAM, habilitar APIs, activar facturación, aplicar Foundation, SQL, Run o DNS. La configuración remota propuesta cambia el destino futuro del backend bootstrap; la migración real queda bloqueada hasta la liberación específica.
