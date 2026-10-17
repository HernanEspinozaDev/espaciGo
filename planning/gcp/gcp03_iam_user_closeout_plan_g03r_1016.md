# GCP-03 IAM login closeout — g03r-1016

**Estado actual:** etapa 0 aplicada desde el plan aprobado en PR #381; etapa 1 preparada para revisión y **no aplicada**. No se ejecutaron sentencias SQL ni Cloud Run Jobs, no se borraron recursos ni objetos y no se alteraron memberships.

## Etapa 0 aplicada

PR #381 (`872b2ee66516c6d8b7f36febfd00d6aaa6d4575e`) fue fusionado por `HernanEspinozaDev`, merge `d954217036066517880881da2c8588cdf0dfe731`. Se aplicó únicamente el plan aprobado SHA-256 `ffcf28449f3b4cada393753c58dc8fc455868d8f41eaaba8f801d2a38f9a4f31`, desde el `main` actualizado al merge. Terraform informó **0 altas, 2 cambios, 0 eliminaciones**. La única diferencia por recurso fue `deletion_policy: PREVENT → DELETE` en `google_sql_user.restore_iam[0]` y `google_sql_user.verify_iam[0]`.

El backend mantiene lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`; serial avanzó de 10 a 11. La verificación posterior confirmó por Cloud SQL Admin API que ambos usuarios siguen existiendo, y por state que conservan IDs, `database_roles` y tipo originales. La instancia sigue `RUNNABLE` con `deletionProtectionEnabled=true`. Los seis Jobs siguen presentes y su state Terraform conserva `deletion_protection=true`. La evidencia saneada está en [iam-user-closeout-stage0-apply.json](evidence/gcp03-restore-g03r-1016/iam-user-closeout-stage0-apply.json).

La etapa 0 no emitió DML/DDL hacia PostgreSQL ni llamó a Cloud SQL Users Update/Delete; el cambio de `deletion_policy` es cliente/state-side en el provider. Las lecturas posteriores usaron Cloud SQL Admin API, SQL API de instancias y listado de Cloud Run Jobs. No se invocaron Jobs ni se realizaron pruebas cloud. El state remoto sí se actualizó de serial 10 a 11 como parte del apply autorizado.

## Etapa 1 preparada para revisión

El plan nuevo se generó después del apply, contra lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 11, desde la rama creada en `main` fusionado. Configuración: `enable_restore_iam_users=false`, `allow_restore_iam_user_destroy=true`, `allow_restore_destroy=false`, `confirm_destroy_run_id="g03r-1016"` y `enable_restore_jobs=true`. Solo se usaron targets `google_sql_user.restore_iam[0]` y `google_sql_user.verify_iam[0]`.

El resumen es **0 altas, 0 cambios, 2 eliminaciones**. Ambos `before.deletion_policy` son `DELETE`, y el [validador de planes](../../scripts/gcp03_iam_user_plan_validator.py) aceptó el JSON real. SHA-256 del binario privado: `e6a04d3e301dcf82db3d82e2d8430637e71eb4e2d7793680ce2a3ef1a4ffa166`. Terraform marca `complete=false` por el uso dirigido de `-target`. El resumen saneado está en [iam-user-closeout-stage1-plan.json](evidence/gcp03-restore-g03r-1016/iam-user-closeout-stage1-plan.json).

**La etapa 1 requiere revisión y no se ha aplicado.** El validador exige exactamente esos dos deletes y rechaza cualquier delete si `before.deletion_policy` no es `DELETE`. Pruebas locales del validador: 5/5. No usar `state rm`, editar state, migrar state ni omitir la comprobación del provider.

Los binarios, JSON completos, tfvars y copias de state se mantienen excluidos de Git en `.local/testing/gcp03-1017/`, con permisos `0600`. El plan de etapa 1 no está allí para aplicar implícitamente: cualquier aplicación requiere revisión independiente del plan y hash.

## Plan previo supersedido

El plan delete/delete con SHA `9afa804a88aefe09523308a8f9698edf47417b97a90c59a4a5fc24d141355c9f` se conserva en el subdirectorio privado `superseded/`: ambos deletes tenían `before.deletion_policy=PREVENT`. El código del provider Google 8.7.0 rechaza el borrado con `PREVENT` antes de llamar a Cloud SQL ([`DeletionPolicyPreDelete`](https://github.com/hashicorp/terraform-provider-google/blob/v8.7.0/google/tpgresource/utils.go#L2613-L2628), [`resourceSqlUserDelete`](https://github.com/hashicorp/terraform-provider-google/blob/v8.7.0/google/services/sql/resource_sql_user.go#L3034-L3046)). No se aplicó ese plan.

## Memberships y seguimiento GCP-04

El diagnóstico de solo lectura de PR #380 encontró `membership_count=2` en ambos usuarios, mientras el código de `retire` exige `membershipCount==1`. No se relaja a `>=1` ni se toca membership desconocida; el hallazgo no demuestra qué subpaso causó `PHASE_FAILED`. La diferencia entre `lastAttemptResult.status={}` en la API de tareas Cloud Run y los códigos de salida de contenedor también permanece como seguimiento acotado GCP-04. No amplía aceptación ni se presenta como resuelto.

## Recursos fuera de alcance

La etapa 1 conserva los roles/grants y memberships; solo retira los dos objetos de usuario Cloud SQL si se revisa y aplica más adelante. Siguen preservados service accounts, Jobs, instancia, secreto y versión, backend Terraform, `pilot01`, fuente persistente y sus backups/archivos, red y subred basales. No se eliminó ni solicitó borrar ninguno de los cuatro objetos del bucket:

| Objeto | Generación |
| --- | ---: |
| `runs/g03r-1016/manifest.json` | `1792195760678849` |
| `runs/g03r-1016/synthetic-v1-v61.dump` | `1792195769229365` |
| `runs/g03r-1016/dump.toc` | `1792195778303427` |
| `runs/g03r-1016/allowlist.toc` | `1792195786699241` |

El borrado de esos objetos necesita autorización explícita y precondiciones idénticas para `generation` e `ifGenerationMatch`. Bajar protecciones, eliminar recursos restantes y borrar objetos son etapas separadas. La aceptación de producto GCP-03/GCP-04 sigue pendiente.

La ventana registrada termina el `2026-10-17T05:08:13.708Z`; no hay autosuspend configurado y el presupuesto/crédito restante no se verificó. La referencia anotada para la instancia es aproximadamente USD 1.39960544 por ocho horas antes de cargos adicionales; véase el [informe de preparación](gcp03_restore_prep_g03r_1016.md).
