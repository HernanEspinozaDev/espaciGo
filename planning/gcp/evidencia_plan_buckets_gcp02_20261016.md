# Evidencia del plan privado de buckets GCP-02

Fecha: 2026-10-16 (UTC). Repositorio y rama: EspaciGo, `codex/infra/chore/gcp-02-state-bootstrap`. Base de código: `origin/main` tras PR #328 (`2b0a173d6aeed9b377c52a488f5d7aacd80a012f`), seguida por esta rama temporal.

## Lecturas cloud previas (solo lectura)

Proyecto objetivo `espacigo-testprod` y región `southamerica-west1`. `storage.googleapis.com` figuró `ENABLED` al consultar con proyecto y billing project explícitos. `gcloud storage buckets list` no mostró buckets de ese proyecto. Los tres `describe` candidatos devolvieron 404:

- `espacigo-testprod-tfstate-447373903153-bootstrap`
- `espacigo-testprod-tfstate-447373903153-gcp`
- `espacigo-testprod-tfstate-447373903153-cloudflare`

Los 404 indican que los nombres no existían al verificarlos; no reservan ni garantizan disponibilidad global al momento de crear. No se ejecutó ninguna escritura cloud.

## Plan Terraform autenticado, no aplicado

`terraform init` usó backend local con path bajo `.local/` y `TF_DATA_DIR` privado; `terraform validate` pasó. Plan `terraform plan -refresh=true`, provider Google 8.7.0, state de recursos vacío separado, variables privadas. Resultado verificado desde JSON: tres recursos `google_storage_bucket.terraform_state` con acción `create`; cero otros recursos, cero IAM/API, **3 to add, 0 to change, 0 to destroy**. UBLA, Public Access Prevention enforced, Standard regional, versioning y `prevent_destroy` quedan configurados por código. El gate `storage_api_preverified=true` solo se usó tras la lectura de estado del API; no habilita el API ni sustituye permisos/revisión de apply.

SHA-256 del plan binario privado: `963e5bcfa01cc0f4b3a0f386c50758f00780067d1265cacd9e3e42af502b270f`.

El `.tfplan`, JSON/texto, tfvars, `TF_DATA_DIR` y backend path privado se conservan bajo `.local/planning/gcp/bootstrap-state-plan-20261016/` (directorio 0700, archivos 0600; excluidos por `.gitignore`). No se incluyó el plan binario ni variables en GitHub. El archivo de recursos `bootstrap.tfstate` no se creó porque no se aplicó. El metadato local de backend que Terraform denomina `terraform.tfstate` permanece únicamente dentro del `TF_DATA_DIR` privado. No hay archivos `*tfstate*` bajo el root bootstrap. Se comprobó `git check-ignore` para plan/tfvars/state.

No se ejecutó `terraform apply`, `import`, cambio IAM, habilitación de servicios, creación de bucket, modificación de billing ni operación de datos. El state de `inventory-apis` previo se conserva sin modificación.
