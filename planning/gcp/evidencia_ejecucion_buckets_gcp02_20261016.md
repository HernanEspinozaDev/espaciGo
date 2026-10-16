# Evidencia de ejecución y verificación de buckets GCP-02

Fecha: 2026-10-16 (UTC). Repositorio `HernanEspinozaDev/espaciGo`. La preparación de código y plan fue fusionada en PR #329, merge `7cb72e06378a01a45c6c93ab447d1109353bc5f6`, por `HernanEspinozaDev`. La operación posterior partió de ese `origin/main` en una rama documental/operativa temporal. SHA del plan previamente revisado y aplicado: `963e5bcfa01cc0f4b3a0f386c50758f00780067d1265cacd9e3e42af502b270f`.

## Preflight y aplicación exacta

Antes de aplicar se revalidaron, con proyecto explícito:

- Proyecto `espacigo-testprod`, número `447373903153`, estado `ACTIVE`.
- La identidad activa coincidió byte por byte y por SHA-256 con la captura privada que se usó en la fase anterior. Un primer cálculo transitorio añadió un salto de línea al hash y se descartó; no ocurrió cambio de cuenta.
- `storage.googleapis.com` seguía `ENABLED`.
- Los tres nombres no existían justo antes de la creación.
- El archivo `.tfplan` coincidía con el hash aprobado, estaba en `.local` (directorio 0700/archivo 0600) y contenía únicamente las tres acciones `create` autorizadas en `southamerica-west1`, con UBLA, PAP enforced y versioning activados.

Se aplicó ese archivo guardado con Terraform 1.16.6 y Google provider 8.7.0. Resultado de Terraform: **3 added, 0 changed, 0 destroyed**. No se ejecutó un plan diferente ni se incluyeron acciones de IAM/API.

Buckets creados:

- `espacigo-testprod-tfstate-447373903153-bootstrap`
- `espacigo-testprod-tfstate-447373903153-gcp`
- `espacigo-testprod-tfstate-447373903153-cloudflare`

## Verificación después de aplicar

Lecturas autenticadas de Storage verificaron para cada bucket: región `SOUTHAMERICA-WEST1`, UBLA activado, prevención de acceso público `enforced` y versioning activado. La consulta IAM de cada bucket y Cloud Asset IAM search encontraron los pseudo-principals de proyecto asociados a los roles legacy de bucket/objeto; no encontraron `allUsers`, `allAuthenticatedUsers`, usuarios directos ni service accounts directas en la política de esos buckets. No se agregaron grants nuevos.

Cloud Asset IAM search a nivel de proyecto devolvió una política de proyecto con un único binding `roles/owner` y sin principals públicos. La consulta Cloud Resource Manager `projects get-iam-policy` no pudo completarse porque su API está deshabilitada y la identidad carece del permiso para esa lectura. La búsqueda IAM en la organización `644690775054` fue rechazada por permisos. No se habilitó `cloudresourcemanager.googleapis.com` ni se ampliaron permisos. Por ello, la evidencia confirma las políticas visibles del bucket y el binding de proyecto visible mediante Cloud Asset, pero no permite enumerar o verificar la herencia completa desde la organización. Respuestas detalladas y errores se conservan en privado.

Se comprobó también que cada bucket sigue vacío. Un plan Terraform posterior con refresh registró las tres direcciones como `no-op`: **0 add, 0 change, 0 destroy**. El state bootstrap local está en `.local/planning/gcp/bootstrap-state-plan-20261016/bootstrap.tfstate`, modo 0600; directorio privado modo 0700. `TF_DATA_DIR`, planes y respuestas IAM se mantienen en el mismo directorio excluido de Git. No se migró ni movió el state. El state previo `inventory-apis` y su snapshot se conservaron sin modificación.

La estimación de costos fue corregida en esta revisión para separar Cloud Run API instance-based y SSR request-based; el documento del 2026-10-15 se conserva como evidencia histórica y el modelo actual está en `infra/terraform/cost-model.md`.

## Alcance y límites

No se añadieron IAM bindings, no se habilitaron APIs, no se cambió billing, no se aplicaron foundation, Cloud SQL, Cloud Run, DNS, bases de datos ni cambios de dataset. No se cargó state en los buckets y no se inició la migración de backend. El acceso de la identidad de operación futura deberá revisarse y concederse explícitamente por bucket en una fase posterior aprobada; esos grants no forman parte de esta ejecución.
