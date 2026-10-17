# GCP-03 IAM login closeout plan — g03r-1016

**Estado: plan dirigido preparado para revisión; no aplicado.** Esta preparación no ejecutó SQL ni Cloud Run Jobs, no borró usuarios ni objetos y no hizo escrituras en GCP. El resultado de `terraform plan` puede aplicarse técnicamente, pero Terraform lo marca `complete=false` porque usa `-target`; no es un plan completo de retiro.

## Plan de fase 1

El plan se generó con Terraform 1.16.6 y Google provider 8.7.0 contra el backend remoto actualizado: lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 10. Su SHA-256 es `9afa804a88aefe09523308a8f9698edf47417b97a90c59a4a5fc24d141355c9f`. El resumen validado desde el JSON del plan es **0 altas, 0 cambios, 2 eliminaciones**:

- `google_sql_user.restore_iam[0]` — `g03-restore-g03r-1016@espacigo-testprod.iam`.
- `google_sql_user.verify_iam[0]` — `g03-verify-g03r-1016@espacigo-testprod.iam`.

Los únicos targets fueron esas dos direcciones. El JSON mostró no-op para las cuentas de servicio restore/verify y la instancia Cloud SQL. La configuración conserva `enable_restore_jobs=true`; los Jobs, secreto y versión, backend, red basal, `pilot01` y dataset persistente están fuera de los targets y siguen en el estado del run. Al ser dirigido, este plan no certifica una revisión completa de esos recursos. La compuerta `enable_restore_iam_users=false` requiere el gate de destrucción exclusivo de usuarios y la confirmación exacta `g03r-1016`; no se usó `allow_restore_destroy`.

La evidencia saneada y hash-bound está en [iam-user-closeout-plan.json](evidence/gcp03-restore-g03r-1016/iam-user-closeout-plan.json). El binario `.tfplan`, su JSON completo, las variables, la copia privada de state y los logs permanecen en `.local/testing/gcp03-1017/` con permisos `0600`; no se publican en Git. El plan anterior serial 9 queda supersedido y no debe aplicarse.

## Hallazgo de memberships y seguimiento GCP-04

El diagnóstico de solo lectura incluido en PR #380 encontró que los dos usuarios IAM mantienen la membership esperada y tienen `membership_count=2`. El código de `retire` exige `membershipCount==1`. No se cambia ese contrato a `>=1` ni se manipulan memberships cuyo origen no está identificado. El hallazgo es una incompatibilidad que requiere seguimiento; **no demuestra cuál subpaso causó el `PHASE_FAILED` anterior**.

Se registran también como seguimiento acotado de GCP-04 la discrepancia entre `lastAttemptResult.status={}` en la API de tareas de Cloud Run y los códigos de salida vistos en logs, y el contrato de cardinalidad `membershipCount==1`. Estos puntos no amplían la aceptación de GCP-04 ni se presentan como resueltos.

## Límites y etapas pendientes

La ejecución previa `retire` reportó `PHASE_FAILED`; el diagnóstico no prueba que su retiro haya tenido éxito. Por ello, este PR presenta el plan para revisión y **no autoriza su aplicación**. Tampoco concede una nueva ejecución de `retire`.

Solo después de una revisión que libere expresamente esta fase se podría aplicar el plan exacto de dos usuarios y verificar la ausencia de ambos mediante la API de Cloud SQL. Los siguientes trabajos requieren planes y revisiones separados:

1. preparar un plan separado para bajar protecciones de los recursos restantes del run;
2. preparar otro plan para retirar Jobs, bindings, cuentas de servicio, secreto, base, instancia y bucket del run, con una lista de acciones revisada;
3. tratar los cuatro objetos listados más abajo únicamente después de autorización explícita para borrado condicional por nombre y generación exactos.

No se ha iniciado teardown. Se conservan los cuatro objetos y generaciones observados:

| Objeto | Generación |
| --- | ---: |
| `runs/g03r-1016/manifest.json` | `1792195760678849` |
| `runs/g03r-1016/synthetic-v1-v61.dump` | `1792195769229365` |
| `runs/g03r-1016/dump.toc` | `1792195778303427` |
| `runs/g03r-1016/allowlist.toc` | `1792195786699241` |

Cualquier futuro borrado deberá volver a leer metadatos y usar `generation` e `ifGenerationMatch` iguales al valor aprobado. Esta preparación no envió solicitudes de borrado.

Permanecen preservados el baseline persistente y sus backups/archivos, `pilot01`, el state backend, la red y subred compartidas, y los recursos que no pertenecen al run. La ventana indicada para este seguimiento termina el `2026-10-17T05:08:13.708Z`; no hay autosuspend configurado y el presupuesto o crédito restante no se verificó. La referencia de costo de Cloud SQL anotada para la instancia de este tamaño es aproximadamente USD 1,39960544 por ocho horas, antes de backups, red, logs, impuestos y descuentos; ver el [informe de preparación](gcp03_restore_prep_g03r_1016.md).

## Verificación local y límites

`terraform fmt -check`, `terraform validate`, `terraform test` (9/9) y `git diff --check` pasaron localmente. No se ejecutaron pruebas cloud, SQL ni Jobs. La generación del plan hizo refresh de lectura y produjo dos acciones `delete`; no se ejecutó `terraform apply`. La aceptación de producto GCP-03 y GCP-04 sigue pendiente.
