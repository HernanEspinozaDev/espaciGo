# GCP-03 run closeout sequence — g03r-1016

**Estado actual:** stage1 eliminó los dos usuarios SQL IAM, stage2 bajó las protecciones, se borraron las cuatro generaciones autorizadas de objetos y stage3 retiró los 30 recursos restantes del run mediante el plan revisado. El state conserva lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 16 y cero recursos gestionados. Un plan completo y refrescado posterior no detectó cambios ni drift. Esto cierra la limpieza cloud de `g03r-1016`, no GCP-03 ni su aceptación de producto.

## Etapa 0: `deletion_policy` de usuarios

PR #381 (`872b2ee66516c6d8b7f36febfd00d6aaa6d4575e`) fue fusionado por `HernanEspinozaDev`, merge `d954217036066517880881da2c8588cdf0dfe731`. Se aplicó el plan aprobado SHA-256 `ffcf28449f3b4cada393753c58dc8fc455868d8f41eaaba8f801d2a38f9a4f31`, con 0 altas, 2 cambios y 0 eliminaciones. Solo cambió `deletion_policy: PREVENT → DELETE` en los usuarios restore/verify. State lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 10→11.

## Etapa 1: eliminación de los dos usuarios

PR #382 (`18ecda24506871112c170ebdcfada087b1e4ca22`) fue fusionado por `HernanEspinozaDev`, merge `e6c1d5d3f5aac2c05ca7a1ae3022577f2a072c47`. Se aplicó exactamente el plan aprobado SHA-256 `e6a04d3e301dcf82db3d82e2d8430637e71eb4e2d7793680ce2a3ef1a4ffa166`: 0 altas, 0 cambios y 2 eliminaciones.

Cloud SQL Admin API confirmó las dos operaciones `DELETE_USER` como `DONE`, ambas con `error=null`. La API ya no lista los usuarios y el state serial 12 no contiene recursos `google_sql_user`. La instancia sigue RUNNABLE con protección nativa habilitada; los seis Jobs siguen presentes y protegidos; el secreto y versión 1 continúan presentes/habilitados. Los cuatro objetos y generaciones siguen presentes según una lectura posterior de todas las versiones del prefijo. La evidencia saneada está en [iam-user-closeout-stage1-apply.json](evidence/gcp03-restore-g03r-1016/iam-user-closeout-stage1-apply.json).

La etapa 1 usó únicamente Cloud SQL Delete API a través del provider; no se ejecutaron sentencias SQL ni Jobs, y no se hizo un segundo intento. No hubo errores ni resultados inciertos.

### Corrección de evidencia stage0

La evidencia stage0 publicada anteriormente contenía un typo en `verify.id`: omitía `@espacigo-testprod.iam//`. En este PR se corrige el valor a `g03-verify-g03r-1016@espacigo-testprod.iam//espacigo-gcp03-restore-g03r-1016-pg` y se registra explícitamente el valor anterior y el corregido en el campo `corrections`. El state y plan reales siempre tuvieron el ID corregido.

## Etapa 2: plan de protecciones y aplicación revisada

El plan se generó tras stage1, desde el `main` actualizado al merge de PR #382, contra state lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 12. Fue completo y no dirigido. Inputs relevantes: `enable_restore_iam_users=false`, `allow_restore_iam_user_destroy=true`, `allow_restore_destroy=true`, `confirm_destroy_run_id="g03r-1016"`, `enable_restore_jobs=true`.

El binario privado tiene SHA-256 `e89e9c70ee79ca1b5b88a025d1a2c711fcece2b3485957cc9cd5fac3bf9893ff`. Resultado: **0 altas, 9 cambios, 0 eliminaciones**. El cambio se limita a:

- Cloud SQL instance: `deletion_protection=true→false`, `deletion_policy=PREVENT→DELETE` y `settings.deletion_protection_enabled=true→false`.
- Base `espacigo_scratch`: `deletion_policy=PREVENT→DELETE`.
- Seis Cloud Run Jobs: `deletion_protection=true→false`.
- Secret metadata: `deletion_protection=true→false` y `deletion_policy=PREVENT→DELETE`; el secreto y la versión se mantienen.

No se recrean usuarios SQL IAM: `enable_restore_iam_users=false`, el state no contiene esos recursos y el plan no los crea ni los modifica. El validador [gcp03_run_protection_plan_validator.py](../../scripts/gcp03_run_protection_plan_validator.py) aceptó el JSON real y exige el conjunto exacto de recursos/campos, sin altas ni eliminaciones. Sus siete pruebas locales pasan. Evidencia saneada: [run-protection-stage2-plan.json](evidence/gcp03-restore-g03r-1016/run-protection-stage2-plan.json).

**Stage2 fue aprobado y fusionado en PR #383 (HEAD `f09d15147da3b6c9749e55790bd071925be3edc3`, merge `aa71bf6b7b377c584cce91634d68d2d0a97556dd`) y se aplicó únicamente el binario aprobado con SHA anterior. Terraform informó 0 altas, 9 cambios y 0 eliminaciones; state avanzó a serial 13. El refresco posterior tuvo 0 cambios y 0 drift. Usuarios SQL IAM permanecen ausentes; la instancia quedó RUNNABLE con protección deshabilitada; los seis Jobs siguen presentes y desprotegidos; el secreto y su versión 1 `ENABLED` siguen presentes. Véase [evidencia stage2 apply](evidence/gcp03-restore-g03r-1016/run-protection-stage2-apply.json).

## Limpieza de artefactos autorizada y Stage3 aplicado

Tras verificar Stage2 se eliminaron únicamente los cuatro objetos exactos del bucket `espacigo-gcp03-restore-g03r-1016`, usando para cada uno el mismo valor en el selector `generation` y `ifGenerationMatch`. El inventario previo coincidió con las cuatro generaciones autorizadas; Cloud Storage JSON API confirmó el prefijo y el bucket sin versiones/objetos restantes y cada consulta por generación devuelve 404. Los nombres, generaciones y hashes MD5/CRC32C están en [evidencia de limpieza](evidence/gcp03-restore-g03r-1016/run-artifact-cleanup.json). El bucket en sí no se eliminó.

El plan stage3 se generó completo, sin `-target`, desde el merge #383 y state serial 13. SHA-256 del binario privado: `ea47a00c6330bf006b2f5e5abf2fc376f399b1cdaa428677bd089cbd40b753cc`. Incluye **0 altas, 0 cambios y 30 eliminaciones**, todas de recursos gestionados del run `g03r-1016`; no incluye usuarios SQL IAM ni drift. Foundation y los cuatro gates de bootstrap, restore, diagnóstico y diagnóstico de retiro están desactivados; las confirmaciones de setup también están en falso. El bucket estaba vacío antes del plan. El validador exacto de inventario/gates y grafo aceptó el plan; sus ocho pruebas locales pasan. Evidencia saneada: [Stage3 plan](evidence/gcp03-restore-g03r-1016/stage3-teardown-plan.json) y [grafo de dependencias](evidence/gcp03-restore-g03r-1016/stage3-teardown-plan-graph.dot).

**Stage3 se aplicó una vez con el plan exacto revisado.** PR #384 fue aprobado y fusionado por `HernanEspinozaDev` con HEAD revisado `13e3ecb1fdafe6111d277761b6a2480a5c276743`, merge `e50fd4bb526bdd04992b00e8e807271345214f39`. Se aplicó el SHA anterior sin regenerar el plan: Terraform informó 0 altas, 0 cambios y 30 eliminaciones. El state avanzó de serial 13 a 16, conservando lineage, y quedó sin bloques gestionados. La evidencia saneada está en [Stage3 apply](evidence/gcp03-restore-g03r-1016/stage3-teardown-apply.json); el plan posterior completo/refrescado tuvo cero cambios y cero drift y no se aplicó ([evidencia](evidence/gcp03-restore-g03r-1016/stage3-postapply-nochange.json)).

Durante la eliminación, una consulta adicional de solo lectura a Cloud SQL Operations devolvió un mensaje de permiso/no existe y no pudo establecer el contenido de la lista mientras el borrado estaba en curso. No se reintentó; la consulta no modificó recursos. Terraform continuó y terminó con éxito. Las verificaciones posteriores confirmaron la ausencia de los recursos run-scoped y conservaron las instancias base y `pilot01` en `RUNNABLE`; no se ejecutó SQL ni Jobs durante el teardown.

Permanecen fuera de alcance `pilot01`, el dataset persistente de origen y sus backups/archivos, red/subred basales, recursos compartidos y backend Terraform. La API confirmó que el secreto, instancia/base, Jobs, service accounts y bucket del run están ausentes; se conservaron el bucket y objeto del backend, con versionado habilitado. No se ejecutaron sentencias SQL ni Jobs durante la etapa 3. La limpieza de objetos previa se limitó a las cuatro generaciones autorizadas.

El hallazgo GCP-04 de `membershipCount==1` frente al diagnóstico `membership_count=2` no se relajó ni se alteraron memberships desconocidas. La discrepancia de `lastAttemptResult.status={}` de Cloud Run y los códigos de salida en logs sigue registrada como seguimiento acotado; no amplía aceptación. La aceptación de producto GCP-03/GCP-04 sigue pendiente.

No se ejecutaron pruebas cloud en esta etapa de preparación. Las lecturas de verificación fueron APIs de Cloud SQL Admin, Secret Manager, Cloud Storage y Cloud Run. No hay autosuspend configurado; el presupuesto/crédito restante no se verificó.
