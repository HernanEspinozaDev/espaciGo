# GCP-03 retirement teardown plan — g03r-1016

**Estado: preparación y revisión solamente; ningún teardown fue aplicado.** El Job `retire` se ejecutó una vez, pero su payload de aplicación reportó `PHASE_FAILED` en `phase_execute`. Cloud Run `Completed=True` no concilia el fallo de aplicación: la investigación posterior encontró salida `1` del contenedor `retire`, salida `0` del proxy y un `lastAttemptResult.status` vacío. El antiguo `commit_outcome_uncertain=false` era el valor booleano por defecto y no demuestra rollback. No se reintentará `retire` como parte de este plan. La investigación y el diagnóstico de solo lectura preparado están en [gcp03_retirement_observability_20261017.md](gcp03_retirement_observability_20261017.md).

## Run y evidencia

- Proyecto/región: `espacigo-testprod` / `southamerica-west1`.
- Run: `g03r-1016`; destino `espacigo-testprod:southamerica-west1:espacigo-gcp03-restore-g03r-1016-pg/espacigo_scratch`.
- [Evidencia sanitizada de retire](evidence/gcp03-restore-g03r-1016/retire-execution.json), SHA-256 `1e9bc62ea23585e20d7138cec1f1b94414591fe13d754257a237ea70827248f2`.
- Restore/verify del fixture sintético mínimo pasaron y están registrados en [restore-verify.json](evidence/gcp03-restore-g03r-1016/restore-verify.json). Ese fixture no es snapshot ni migración del dataset persistente de origen.
- No se borraron usuarios, objetos, secretos, Jobs, cuentas de servicio, base o instancia.

## Plan reemplazado y estado vigente

La attestation histórica [teardown-phase1-attestation.json](evidence/gcp03-restore-g03r-1016/teardown-phase1-attestation.json) registra un plan previo basado en serial 9; ese binario quedó supersedido y no debe aplicarse. El plan vigente para los usuarios SQL IAM está documentado por separado en [GCP-03 IAM login closeout plan](gcp03_iam_user_closeout_plan_g03r_1016.md): etapa 0, contra serial 10, con SHA-256 `ffcf28449f3b4cada393753c58dc8fc455868d8f41eaaba8f801d2a38f9a4f31`. Cambia solamente ambas políticas `deletion_policy` de `PREVENT` a `DELETE`; no elimina usuarios. El plan dirigido, parcial y **no aplicado** con SHA `9afa804a88aefe09523308a8f9698edf47417b97a90c59a4a5fc24d141355c9f` también está supersedido porque ambos deletes tenían `before.deletion_policy=PREVENT`.

La etapa 0 solo actualiza las políticas de ambos usuarios. La etapa 1 de eliminación todavía no se preparó: requiere primero revisión y aplicación de etapa 0 y luego un nuevo plan cuyo validator exija `before.deletion_policy=DELETE`. No incluye la eliminación de los cuatro objetos enumerados en la attestation histórica; sus generaciones se preservan y cualquier borrado posterior requiere autorización explícita y precondiciones `generation`/`ifGenerationMatch` exactas. No se ejecutó teardown. Los planes para bajar otras protecciones y eliminar los recursos restantes son etapas independientes. `pilot01`, la fuente persistente, sus backups/archivos, la red/subred base y el backend permanecen fuera de alcance.

No hay aceptación de producto implícita en este documento ni en la fusión del PR.
