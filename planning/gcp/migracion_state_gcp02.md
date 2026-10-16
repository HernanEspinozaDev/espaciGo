# Procedimiento futuro de migración de Terraform state GCP-02

Estado: plan documental; no autorizado ni ejecutado. Los tres buckets ya existen y no tienen objetos. El state de bootstrap y el state de `inventory-apis` siguen locales, privados y separados. Este documento no reserva ni inicia una migración.

## Antes de migrar

1. Obtener autorización específica para cada migración y revisión del código/backend de esa raíz.
2. Verificar proyecto `espacigo-testprod`, número `447373903153`, identidad activa, API de Storage y bucket objetivo. Confirmar que IAM del bucket permite a la identidad prevista leer/escribir el objeto de state. Cualquier binding debe ser un cambio separado revisado y solo sobre el bucket correspondiente.
3. Respaldar el state local objetivo, calcular SHA-256, comprobar directorio 0700 y archivo 0600, y guardar backup privado bajo `.local/`. Conservar el state de `inventory-apis`; no mezclarlo con bootstrap/GCP/Cloudflare.
4. Usar `umask 077`, `TF_DATA_DIR` privado en `.local`, backend configs privados modo 0600, y confirmar que archivos `terraform.tfstate*` no salen de `.local`.

## Migración bootstrap (solo una vez autorizada)

1. En una revisión separada, cambiar explícitamente `backend "local" {}` de bootstrap a `backend "gcs" {}`; configuración requerida: bucket `espacigo-testprod-tfstate-447373903153-bootstrap` y prefijo propio, privado y estable (p. ej. `bootstrap/terraform.tfstate`). No habilitar otro recurso ni gestionar API Storage desde este root.
2. Con la raíz revisada, inicializar Terraform con `-backend-config` privado y `-migrate-state`, mostrando el plan de copia al usuario antes de confirmar la migración si el CLI lo solicita. No usar `-reconfigure` para descartar el backend local.
3. Verificar remoto el objeto/prefijo, leer state desde GCS, comparar número de recursos y SHA-256 de un backup exportado de forma privada; ejecutar plan autenticado con refresh y confirmar 0 add/change/destroy.
4. Mantener un backup local 0600 hasta confirmar recuperación y revisión; no borrar el state de origen como parte de la migración inicial. Definir retención/versioning/lifecycle del objeto de state antes del uso sostenido.

## Raíces GCP y Cloudflare

- Inicializar `infra/terraform/environments/testprod/gcp` solamente con su bucket `...-gcp` y prefijo propios, mediante backend config privado. Nunca apuntar a bootstrap ni mover su state.
- Inicializar Cloudflare solamente con su bucket `...-cloudflare` y prefijo propios. Compartir solo variables operativas aprobadas, no state GCP ni credenciales cloud.
- Mantener separado el state de `inventory-apis` en `.local/planning/gcp/inventory-apis-exec-20261015/`; no migrarlo ni reconstruirlo en ninguno de estos backends.

## Validaciones y límites

Cada raíz tiene plan y aprobación propios. La primera validación tras migrar cada raíz debe ser `terraform state list`, revisión de bucket/prefijo y plan sin cambios. No migrar varias raíces en una sola operación. No asumir que la creación de buckets concede acceso a operadores; no agregar bindings automáticamente. Detenerse si cambia identidad/proyecto, si existe el objeto de destino, hay conflicto de state, el plan difiere o se requiere un API, IAM binding u otro recurso no aprobado. Esta fase documental no activa facturación pagada ni despliega Foundation, SQL, Run, DNS, bases ni datos.
