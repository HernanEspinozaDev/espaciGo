# GCP-03 retirement teardown plan — g03r-1016

**Estado actual:** stage1 eliminó los dos usuarios SQL IAM; stage2 bajó las protecciones mediante aplicación revisada, y los cuatro objetos autorizados se eliminaron por generación exacta. El plan stage3 de teardown de los recursos restantes está preparado para revisión y **no se aplicó**. En este cierre posterior no se invocaron Jobs; el Job `retire` se ejecutó una vez, pero su payload de aplicación reportó `PHASE_FAILED` en `phase_execute`. Cloud Run `Completed=True` no concilia el fallo de aplicación: la investigación posterior encontró salida `1` del contenedor `retire`, salida `0` del proxy y un `lastAttemptResult.status` vacío. El antiguo `commit_outcome_uncertain=false` era el valor booleano por defecto y no demuestra rollback. No se reintentará `retire` como parte de este plan. La investigación y el diagnóstico de solo lectura preparado están en [gcp03_retirement_observability_20261017.md](gcp03_retirement_observability_20261017.md).

## Run y evidencia

- Proyecto/región: `espacigo-testprod` / `southamerica-west1`.
- Run: `g03r-1016`; destino `espacigo-testprod:southamerica-west1:espacigo-gcp03-restore-g03r-1016-pg/espacigo_scratch`.
- [Evidencia sanitizada de retire](evidence/gcp03-restore-g03r-1016/retire-execution.json), SHA-256 `1e9bc62ea23585e20d7138cec1f1b94414591fe13d754257a237ea70827248f2`.
- Restore/verify del fixture sintético mínimo pasaron y están registrados en [restore-verify.json](evidence/gcp03-restore-g03r-1016/restore-verify.json). Ese fixture no es snapshot ni migración del dataset persistente de origen.
- Los dos usuarios SQL IAM se borraron después de aplicar y verificar las etapas autorizadas 0 y 1. No se borraron objetos, secretos, Jobs, cuentas de servicio, base o instancia.

## Plan reemplazado y estado vigente

La attestation histórica [teardown-phase1-attestation.json](evidence/gcp03-restore-g03r-1016/teardown-phase1-attestation.json) registra un plan previo basado en serial 9; ese binario quedó supersedido y no debe aplicarse. La etapa 0 se aplicó tras PR #381 y la etapa 1 de eliminación tras PR #382; el state avanzó a serial 12, ambos usuarios están ausentes en API y state y las operaciones terminaron `DONE` sin error. Véase [GCP-03 IAM closeout](gcp03_iam_user_closeout_plan_g03r_1016.md) y [evidencia stage1](evidence/gcp03-restore-g03r-1016/iam-user-closeout-stage1-apply.json). El plan delete/delete con SHA `9afa804a88aefe09523308a8f9698edf47417b97a90c59a4a5fc24d141355c9f` permanece supersedido porque ambos deletes tenían `before.deletion_policy=PREVENT`.

La etapa 1 de usuarios se completó y stage2 se aplicó desde PR #383; state serial 13 y refresco posterior sin drift. Luego se eliminaron únicamente las cuatro generaciones autorizadas del prefijo de artifacts; el bucket quedó vacío pero sigue presente. El plan stage3 completo está contra serial 13, incluye 0 altas/0 cambios/30 deletes únicamente del run, fue validado contra el grafo y permanece sin aplicar hasta una nueva revisión. Véanse [evidencia Stage2](evidence/gcp03-restore-g03r-1016/run-protection-stage2-apply.json), [limpieza de objetos](evidence/gcp03-restore-g03r-1016/run-artifact-cleanup.json) y [plan Stage3](evidence/gcp03-restore-g03r-1016/stage3-teardown-plan.json). `pilot01`, la fuente persistente, sus backups/archivos, la red/subred base y el backend permanecen fuera de alcance.

No hay aceptación de producto implícita en este documento ni en la fusión del PR.
