# Bootstrap de APIs de inventario (separado)

Esta raíz administra únicamente `google_project_service` para Organization Policy API y Cloud Asset Inventory API del proyecto existente `espacigo-testprod`. El proyecto está fijado/validado en `variables.tf`; el state/backend se desactiva para la revisión candidata. No crea buckets, grants IAM, políticas, recursos de producto ni exportaciones. `disable_on_destroy=false` evita deshabilitar APIs si una operación futura destruye su state.

## Preflight y plan local

Antes de cada plan, confirmar estado actual con el proyecto explícito y agregar a `services_to_enable` solo APIs que sigan ausentes en la lista `--enabled`:

```sh
gcloud services list --enabled --project=espacigo-testprod --format=json
terraform init -backend=false -input=false -lockfile=readonly
terraform validate
terraform plan -refresh=false -lock=false -input=false -no-color \
  -var='project_id=espacigo-testprod' \
  -var='services_to_enable=["orgpolicy.googleapis.com","cloudasset.googleapis.com"]' \
  -out=/ruta/privada/inventory-apis.tfplan
```

La lista debe ajustarse al preflight inmediatamente anterior. En la revisión del 2026-10-15 America/Santiago, `gcloud services list --enabled --project=espacigo-testprod` confirmó que ambas no estaban habilitadas, por lo que el plan local de esta revisión declara únicamente esas dos. El plan guardado fuera del repositorio es candidata estructural con `-refresh=false`, sin state remoto; no actualiza el estado cloud ni sustituye el preflight antes de un posible apply. Si la API ya está habilitada al revalidar, no incluirla. Mantener sin autorización las operaciones Apply, import, deshabilitar API, o backend remoto.

## Permisos y costo

Para habilitar, requiere `serviceusage.services.enable` en el proyecto; para volver a consultar estado, `serviceusage.services.list`. Google recomienda `roles/serviceusage.serviceUsageAdmin`; si la organización ofrece roles custom, limitarlo a esos permisos, sin Owner ni permisos de edición de políticas. Para la lectura posterior: `orgpolicy.policy.get` en el recurso/proyecto consultado y `cloudasset.assets.searchAllResources` sobre `projects/espacigo-testprod`. No se requieren permisos `orgpolicy.policies.update` ni acceso de escritura a Cloud Asset.

El costo directo de habilitar los dos APIs y hacer lecturas de Organization Policy/Cloud Asset es USD 0; Cloud Asset Inventory declara gratuito su uso. No se crean ni exportan datos a Storage. Los recursos encontrados podrán tener sus propios costos actuales, que este plan no cambia. Referencias: [Service Usage: enable y permisos](https://docs.cloud.google.com/service-usage/docs/enable-disable), [Cloud Asset: permiso de búsqueda](https://docs.cloud.google.com/asset-inventory/docs/reference/rest/v1/TopLevel/searchAll), [precios Cloud Asset](https://cloud.google.com/asset-inventory/pricing).
