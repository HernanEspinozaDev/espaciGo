# Evidencia de habilitación mínima, inventario y políticas

Fecha local: 2026-10-15 (America/Santiago). Intervalo UTC: 2026-10-16 01:18–01:24. Proyecto explícito `espacigo-testprod`, número `447373903153`, organización `644690775054`. La rama parte del merge de GCP-02; no se modificó el proyecto default de gcloud.

## Habilitación acotada

- Preflight autenticado: cuenta activa no cambió, ADC disponible, proyecto/número/organización coinciden. La lista enabled inmediatamente antes del plan confirmó `orgpolicy.googleapis.com` y `cloudasset.googleapis.com` deshabilitadas.
- Raíz Terraform: `infra/terraform/environments/testprod/inventory-apis`. Plan autenticado con refresh, backend local, state privado y únicamente esas dos direcciones `google_project_service`; SHA-256 del plan guardado `db3e7658810f4b24b66c5b3d2a40459dfffd2ce50d11bee9bd0090e1864f7602`.
- Resultado del plan: 2 to add, 0 to change, 0 to destroy. `terraform apply` usó exactamente ese archivo: 2 added, 0 changed, 0 destroyed. No hubo IAM, buckets, Foundation, SQL, Run, DNS, cambios de billing, base de datos ni datos.
- Después, ambas APIs aparecen enabled; las APIs Cloud Resource Manager, Compute, IAM, Run, SQL Admin, Secret Manager y Artifact Registry continúan no habilitadas. `terraform state list` conserva solo las dos APIs y un nuevo plan autenticado devolvió “No changes”.
- Logs Service Usage del intervalo muestran dos operaciones `BatchEnableServices`; sus `serviceIds` incluyen únicamente los dos servicios autorizados. No se observó una dependencia extra en esas solicitudes. El listado enabled posterior contiene 25 servicios; la instantánea previa guardada solo registró el estado de las dos APIs objetivo, no la lista completa, así que no se afirma un delta agregado del total ni se descarta una habilitación interna que no apareciera como servicio solicitado.
- El primer intento de `gcloud asset`/Organization Policy usó el proyecto de cuota default ajeno y pidió habilitar allí las APIs. La respuesta no interactiva fue `N`: no se habilitó API ni se cambió ese proyecto. Las consultas repetidas usaron `--project=espacigo-testprod --billing-project=espacigo-testprod` y completaron.

## Estado y evidencia privada

La primera escritura de Terraform dejó el state local predeterminado en el root con permisos `0664`. Se detectó después del apply, se migró de inmediato a `.local/planning/gcp/inventory-apis-exec-20261015/inventory-apis.tfstate` con modo `0600`, y el respaldo se conservó en el mismo directorio privado también con modo `0600`. El archivo vacío/root fue retirado; `**/terraform.tfstate*` y `.terraform/` quedan ignorados. El root usa backend `local` para que futuros planes/applies apunten a una ruta privada. Ningún state ni credencial se publicó.

Los JSON completos de Asset Inventory, políticas, logs, cuenta activa, plan binario, variables y state están solo en ese directorio `.local` (directorio `0700`, archivos `0600`). La evidencia pública omite identidades, nombres de recursos internos, tokens y valores sensibles.

## Inventario de Cloud Asset

Consulta de solo lectura `gcloud asset search-all-resources --scope=projects/espacigo-testprod`, con máscara de nombres/tipo/ubicación/proyecto/estado: 27 resultados —25 entradas `serviceusage.googleapis.com/Service`, 1 Project y 1 ProjectBillingInfo. Todos los resultados tienen ubicación global. No se exportaron a Storage ni se leyeron datos de aplicación. La API no ofrece una prueba universal de ausencia para tipos no cubiertos por search; superficies específicas aún pueden exigir APIs/permisos de producto.

El acceso de búsqueda funcionó sin conceder IAM: permiso utilizado `cloudasset.assets.searchAllResources` sobre el scope del proyecto. Las instrucciones oficiales enumeran ese permiso para `searchAllResources`; Cloud Asset Inventory informa que su uso es gratuito: [referencia de búsqueda y permiso](https://docs.cloud.google.com/asset-inventory/docs/reference/rest/v1/TopLevel/searchAll), [precios](https://cloud.google.com/asset-inventory/pricing).

## Organization Policy: valores efectivos y origen

Proyecto sin folder intermedio: parent Organization `644690775054`. En `gcloud org-policies list --show-unset`, el scope de proyecto devolvió 201 constraints y 0 políticas directas configuradas; el scope de organización devolvió 201 constraints y 6 políticas directas configuradas. Para las 15 restricciones pertinentes se guardó `gcloud org-policies describe CONSTRAINT --project=espacigo-testprod --effective`. La respuesta efectiva se comparó con las listas directas de proyecto y organización:

| Restricciones | Respuesta efectiva en proyecto | Directa/origen |
| --- | --- | --- |
| `run.allowedIngress`, `run.allowedVPCEgress`, `compute.restrictLoadBalancerCreationForTypes`, `compute.restrictVpcPeering`, `iam.allowedPolicyMemberDomains`, `gcp.resourceLocations`, `serviceuser.services` | `allowAll=true` | Sin policy directa configurada en proyecto ni organización |
| `compute.disableGlobalLoadBalancing`, `iam.disableServiceAccountCreation` | `enforce=false` | Sin policy directa configurada en proyecto ni organización |
| `storage.uniformBucketLevelAccess` | `enforce=true` | Heredada desde policy directa de la organización |
| `compute.restrictProtocolForwardingCreationForTypes` | `allowedValues=["INTERNAL"]` | Heredada desde policy directa de la organización; controla forwarding a target instances, no el tipo de Load Balancer |
| `iam.disableServiceAccountKeyCreation`, `iam.disableServiceAccountKeyUpload`, `iam.automaticIamGrantsForDefaultServiceAccounts` | `enforce=true` | Heredadas desde policies directas de la organización |
| `compute.setNewProjectDefaultToZonalDNSOnly` | `enforce=true` | Heredada desde policy directa de la organización; gobierna defaults al crear proyectos |

La identidad de lectura obtuvo `orgpolicy.policies.list` y `orgpolicy.policy.get` tanto en el proyecto como en la organización; no se cambió IAM ni se concedió `roles/orgpolicy.policyViewer`. No hubo permiso denegado. Para consultas futuras, el rol predefinido de lectura es `roles/orgpolicy.policyViewer`; permisos y roles oficiales: [Organization Policy IAM](https://docs.cloud.google.com/iam/docs/roles-permissions/orgpolicy). `list --show-unset` por sí solo no revela herencia; aquí se complementó con la respuesta effective del proyecto y la lista directa de organización.

## Costos y límites pendientes

La habilitación de APIs y estas lecturas no crearon recursos de cómputo/almacenamiento ni cambiaron billing. Costo directo de esta operación de inventario: USD 0; consultas de Cloud Asset incluidas en el uso gratuito documentado. No se usa este costo como presupuesto del entorno.

Continúan pendientes el costo total de Foundation, servicios y egreso, confirmar SKU/precios regionales y los nombres globales de buckets antes de planificar bootstrap. No se ejecutó el bootstrap de state ni cualquier root GCP/Cloudflare. SQL, Run, DNS, bases, archivos y el dataset de 1.200 tratos permanecen fuera de esta fase; la carga histórica de tratos no se repitió ni regeneró.
