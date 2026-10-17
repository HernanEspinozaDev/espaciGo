# GCP-03 retirement teardown plan — g03r-1016

**Estado: preparación y revisión solamente; ningún teardown fue aplicado.** El Job `retire` se ejecutó una vez, pero su payload de aplicación reportó `PHASE_FAILED` en `phase_execute`. Cloud Run `Completed=True` solo confirma que terminó la tarea; no demuestra que el retiro de aplicación pasara. `commit_outcome_uncertain=false` es el valor emitido por el executor, pero este plan no infiere rollback ni inspeccionó directamente la base después del fallo. No se reintentará `retire` como parte de este plan.

## Run y evidencia

- Proyecto/región: `espacigo-testprod` / `southamerica-west1`.
- Run: `g03r-1016`; destino `espacigo-testprod:southamerica-west1:espacigo-gcp03-restore-g03r-1016-pg/espacigo_scratch`.
- [Evidencia sanitizada de retire](evidence/gcp03-restore-g03r-1016/retire-execution.json), SHA-256 `0dfff91776739691f5d03a4b3b72d568b38da046237fd6d28938b112a92713d9`.
- Restore/verify del fixture sintético mínimo pasaron y están registrados en [restore-verify.json](evidence/gcp03-restore-g03r-1016/restore-verify.json). Ese fixture no es snapshot ni migración del dataset persistente de origen.
- No se borraron usuarios, objetos, secretos, Jobs, cuentas de servicio, base o instancia.

## Fase 1 preparada

Esta fase se limita a los dos usuarios SQL IAM del run y los cuatro objetos de su bucket. **La evidencia de fallo de `retire` es un bloqueo para aplicar la fase 1** hasta que el resultado se investigue y una revisión separada libere una nueva operación. Este PR no concede ese apply.

### 1. Usuarios SQL IAM, antes de cuentas de servicio e instancia

Terraform administra estos dos usuarios de Cloud SQL y puede eliminarlos por la API de Cloud SQL:

- `google_sql_user.restore_iam[0]` — `g03-restore-g03r-1016@espacigo-testprod.iam`.
- `google_sql_user.verify_iam[0]` — `g03-verify-g03r-1016@espacigo-testprod.iam`.

El plan binario privado de fase 1 se generó desde el estado serial 9 en `2eedba7d67cfe8bc4e6c163edf34170c618788c8`. SHA-256: `8e00f5cc08ed82c6b9efd9e56bc012cea233452ac301110126f92a2f6ab5292d`. Su resumen es **0 altas, 0 cambios, 2 eliminaciones**, exactamente las dos direcciones anteriores. Se preparó con `allow_restore_destroy=true`, `run_id="g03r-1016"` y `confirm_destroy_run_id="g03r-1016"` (el nombre real de la variable de confirmación en este root).

Es un plan parcial intencional con `-target`; Terraform informa `complete=false`. El binario se retiene bajo `.local/testing/` con modo 0600 y no se publica. No aplicarlo a ciegas ni si cambia el estado: antes de cualquier aplicación posterior, volver a comprobar estado e identidades y generar/revisar un plan fresco con el mismo alcance exacto.

Después de una eventual aplicación aprobada, comprobar por la API de Cloud SQL que ambos usuarios ya no existen antes de retirar las cuentas de servicio o la instancia. No usar SQL `DROP USER`, `REVOKE` o `NOLOGIN` como sustituto de la eliminación de usuarios IAM.

### 2. Solo los cuatro objetos con generación exacta

Las generaciones observadas en el preflight fueron:

| Objeto | Generación autorizada para identificar |
| --- | ---: |
| `manifest.json` | `1792195760678849` |
| `synthetic-v1-v61.dump` | `1792195769229365` |
| `dump.toc` | `1792195778303427` |
| `allowlist.toc` | `1792195786699241` |

Antes de cada futura eliminación, volver a leer metadatos y abortar si la generación actual no coincide. La operación prevista es Cloud Storage JSON API `objects.delete`, especificando **ambos** `generation=<valor exacto>` e `ifGenerationMatch=<mismo valor>` para ese nombre y bucket `espacigo-gcp03-restore-g03r-1016`; no usar comodines, borrado recursivo ni borrar otras versiones. La API documenta que `generation` selecciona una revisión concreta y `ifGenerationMatch` condiciona la operación a esa generación actual ([referencia oficial](https://docs.cloud.google.com/storage/docs/json_api/v1/objects/delete)). Este PR no envía solicitudes de borrado.

Después de las dos etapas de fase 1, verificar que esos cuatro objetos/generaciones ya no están presentes. No borrar el bucket todavía.

## Fase 2: plan futuro separado

Solo después de completar y revisar la fase 1 se generará un **plan Terraform nuevo y completo**, con `allow_restore_destroy=true` y `confirm_destroy_run_id="g03r-1016"`, para los recursos administrados restantes del run. El estado usado para el inventario privado contiene 30 instancias administradas con prefijo/run `g03r-1016`; los data sources `google_compute_network.existing` y `google_compute_subnetwork.existing` son solo lecturas y quedan excluidos.

El orden deberá mantener los usuarios SQL IAM fuera antes de sus service accounts; retirar bindings IAM y Jobs del run antes de borrar las cuentas de servicio; eliminar la base antes de la instancia; eliminar el binding del secreto antes de su metadata; y borrar el bucket solo después de retirar los cuatro objetos exactos. La metadata y el secreto del run permanecen intactos hasta un plan fase 2 revisado. El grafo y las direcciones efectivas del plan nuevo se comprobarán antes de solicitar cualquier autorización de apply.

Quedan fuera de ambas fases: service accounts y SQL de `pilot01`, el dataset persistente de origen y sus backups/archivos, la red/subred basal, recursos compartidos, el backend de state y cualquier objeto que no sea uno de los cuatro nombres y generaciones enumerados arriba. No se declara GCP-03 completo ni se afirma que el dataset persistente haya sido migrado.

## Attestation

El resumen verificable del plan, sus límites y el inventario de fase 2 están en [teardown-phase1-attestation.json](evidence/gcp03-restore-g03r-1016/teardown-phase1-attestation.json). El plan binario es preparatorio, parcial y **no aplicado**. No hay aprobación de producto implícita en este documento ni en la fusión del PR.
