# GCP-03 IAM login closeout plan — g03r-1016

**Estado: etapa 0 preparada para revisión; no aplicada.** La etapa actual actualiza únicamente la política de borrado de los dos usuarios SQL IAM. No elimina usuarios, no ejecuta SQL ni Jobs y no hace teardown. La eliminación será otra etapa, con otro plan generado solo después de revisar y aplicar la etapa 0.

## Por qué se reemplazó el plan previo

El plan anterior con SHA-256 `9afa804a88aefe09523308a8f9698edf47417b97a90c59a4a5fc24d141355c9f` proponía eliminar ambos usuarios mientras `change.before.deletion_policy` era `PREVENT`. Aunque el plan era técnicamente applyable, el código fuente del provider Google 8.7.0 muestra que `resourceSqlUserDelete` llama primero a `DeletionPolicyPreDelete`, que rechaza el borrado con `PREVENT` y requiere `DELETE`. Ese plan queda supersedido, se conserva únicamente en `.local/testing/gcp03-1017/superseded/` con permisos privados y **no debe aplicarse**. Véase la función oficial [`DeletionPolicyPreDelete`](https://github.com/hashicorp/terraform-provider-google/blob/v8.7.0/google/tpgresource/utils.go#L2613-L2628) y su llamada desde [`resourceSqlUserDelete`](https://github.com/hashicorp/terraform-provider-google/blob/v8.7.0/google/services/sql/resource_sql_user.go#L3034-L3046).

## Etapa 0: plan vigente

State remoto: lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 10. Terraform 1.16.6 y Google provider 8.7.0. El plan se generó con `enable_restore_iam_users=true`, `allow_restore_iam_user_destroy=true`, `allow_restore_destroy=false` y confirmación exacta `g03r-1016`. Solo se usaron como targets:

- `google_sql_user.restore_iam[0]`: `deletion_policy` cambia `PREVENT` → `DELETE`.
- `google_sql_user.verify_iam[0]`: `deletion_policy` cambia `PREVENT` → `DELETE`.

Acciones: **0 altas, 2 cambios, 0 eliminaciones**. Los demás atributos de ambos usuarios no cambian. La validación del JSON confirma exactamente esas dos actualizaciones. SHA-256 del plan binario privado: `ffcf28449f3b4cada393753c58dc8fc455868d8f41eaaba8f801d2a38f9a4f31`. Terraform informa `complete=false` por el uso intencional de `-target`. La evidencia saneada está en [iam-user-closeout-plan.json](evidence/gcp03-restore-g03r-1016/iam-user-closeout-plan.json). Los binarios, JSON completo, variables y copia del state se conservan fuera de Git, en `.local/testing/gcp03-1017/`, modo `0600`.

El [validador del plan](../../scripts/gcp03_iam_user_plan_validator.py) requiere en etapa 0 la transición de política y rechaza otros atributos o recursos mutados. Para la futura etapa 1, **rechaza cualquier delete cuyo `before.deletion_policy` no sea `DELETE`**. Sus cinco pruebas locales pasan: `python3 scripts/test_gcp03_iam_user_plan_validator.py`.

## Etapa 1: aún no preparada

Este PR no contiene un plan de eliminación vigente. Solo después de que etapa 0 sea revisada y aplicada, se deberá actualizar/leer el state y crear un plan nuevo, dirigido a los mismos dos usuarios. El validador deberá confirmar dos acciones `delete`, sin altas ni cambios, y `before.deletion_policy=DELETE` para ambos. Revisar ese plan nuevo por separado antes de cualquier aplicación. No usar `state rm`, editar state, migrar state ni omitir la comprobación del provider.

La ejecución previa de `retire` informó `PHASE_FAILED`; el diagnóstico no prueba que el retiro fuera exitoso. Este PR tampoco autoriza reintentar `retire`.

## Hallazgo de memberships y seguimiento GCP-04

El diagnóstico de solo lectura incluido en PR #380 encontró la membership esperada y `membership_count=2` en ambos usuarios; el código de `retire` exige `membershipCount==1`. No se relaja a `>=1` ni se toca membership de origen desconocido. El hallazgo no prueba qué subpaso causó `PHASE_FAILED`.

Quedan como seguimientos acotados de GCP-04 el contrato de cardinalidad `membershipCount==1` y la discrepancia entre `lastAttemptResult.status={}` en la API de tareas de Cloud Run y los códigos de salida en logs. Estos hallazgos no amplían aceptación ni se presentan como resueltos.

## Recursos y objetos preservados

La etapa 0 no cambia roles, grants, nombres, membresías, cuentas de servicio, Jobs, instancia SQL, secreto o versión, backend de Terraform, red/subred compartida, `pilot01`, fuente persistente ni backups/archivos. Los cuatro objetos y generaciones observados siguen retenidos:

| Objeto | Generación |
| --- | ---: |
| `runs/g03r-1016/manifest.json` | `1792195760678849` |
| `runs/g03r-1016/synthetic-v1-v61.dump` | `1792195769229365` |
| `runs/g03r-1016/dump.toc` | `1792195778303427` |
| `runs/g03r-1016/allowlist.toc` | `1792195786699241` |

No se envió borrado de objetos. Cualquier futura operación requiere autorización explícita y precondiciones iguales para `generation` e `ifGenerationMatch`.

No hubo `terraform apply`, SQL, invocación de Job ni prueba cloud. La ventana registrada termina en `2026-10-17T05:08:13.708Z`; no existe autosuspend configurado y no se verificó presupuesto/crédito restante. La referencia local para esta instancia es aproximadamente USD 1.39960544 por ocho horas antes de otros cargos; véase el [informe de preparación](gcp03_restore_prep_g03r_1016.md). La aceptación de producto de GCP-03/GCP-04 sigue pendiente.
