# Bootstrap de APIs de inventario (separado)

Esta raíz administra únicamente `google_project_service` para Organization Policy API y Cloud Asset Inventory API del proyecto existente `espacigo-testprod`. El proyecto está fijado/validado en `variables.tf`; usa backend local en ruta privada y no state remoto. No crea buckets, grants IAM, políticas, recursos de producto ni exportaciones. `disable_on_destroy=false` evita deshabilitar APIs si una operación futura destruye su state.

## Preflight y plan local

Antes de cada plan, confirmar estado actual con el proyecto explícito y agregar a `services_to_enable` solo APIs que sigan ausentes en la lista `--enabled`:

```sh
gcloud services list --enabled --project=espacigo-testprod --format=json
terraform init -input=false -lockfile=readonly \
  -backend-config='path=/ruta/privada/inventory-apis.tfstate'
terraform validate
terraform plan -refresh=false -lock=false -input=false -no-color \
  -var='project_id=espacigo-testprod' \
  -var='services_to_enable=["orgpolicy.googleapis.com","cloudasset.googleapis.com"]' \
  -out=/ruta/privada/inventory-apis.tfplan
```

La lista debe ajustarse al preflight inmediatamente anterior. Use `-refresh=false` solo para una revisión estructural local; el plan autenticado que pueda preceder a un apply debe usar refresh por defecto, como esta ejecución. En la revisión del 2026-10-15 America/Santiago, `gcloud services list --enabled --project=espacigo-testprod` confirmó que ambas no estaban habilitadas, por lo que el plan de esta ejecución declara únicamente esas dos. El plan guardado fuera del repositorio no actualiza el estado cloud. Si la API ya está habilitada al revalidar, no incluirla. Mantener backend/state local en `.local` privado. No deshabilitar API ni configurar backend remoto como parte de este paso.

## Permisos y costo

Para habilitar/leer el estado de los servicios, requiere `serviceusage.services.enable` y `serviceusage.services.get` en el proyecto; para volver a consultar la lista, `serviceusage.services.list`. Google recomienda `roles/serviceusage.serviceUsageAdmin`; si la organización ofrece roles custom, limitarlo a esos permisos, sin Owner ni permisos de edición de políticas. Para la lectura posterior, requiere `orgpolicy.policy.get` para `describe --effective`, `orgpolicy.policies.list` para `org-policies list`, y `cloudasset.assets.searchAllResources` sobre `projects/espacigo-testprod`. No se requieren permisos `orgpolicy.policies.update` ni acceso de escritura a Cloud Asset.

El costo directo de habilitar los dos APIs y hacer lecturas de Organization Policy/Cloud Asset es USD 0; Cloud Asset Inventory declara gratuito su uso. No se crean ni exportan datos a Storage. Los recursos encontrados podrán tener sus propios costos actuales, que este plan no cambia. Referencias: [Service Usage: enable y permisos](https://docs.cloud.google.com/service-usage/docs/enable-disable), [Cloud Asset: permiso de búsqueda](https://docs.cloud.google.com/asset-inventory/docs/reference/rest/v1/TopLevel/searchAll), [precios Cloud Asset](https://cloud.google.com/asset-inventory/pricing).
