# GCP-03 run closeout sequence — g03r-1016

**Estado actual:** los dos usuarios SQL IAM se eliminaron mediante el plan stage1 aprobado en PR #382. Se verificaron ambas operaciones `DONE` sin error, ausencia en Cloud SQL Admin API y state serial 12. El plan stage2 para bajar protecciones de los recursos restantes está preparado para revisión; **no se ha aplicado**.

## Etapa 0: `deletion_policy` de usuarios

PR #381 (`872b2ee66516c6d8b7f36febfd00d6aaa6d4575e`) fue fusionado por `HernanEspinozaDev`, merge `d954217036066517880881da2c8588cdf0dfe731`. Se aplicó el plan aprobado SHA-256 `ffcf28449f3b4cada393753c58dc8fc455868d8f41eaaba8f801d2a38f9a4f31`, con 0 altas, 2 cambios y 0 eliminaciones. Solo cambió `deletion_policy: PREVENT → DELETE` en los usuarios restore/verify. State lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 10→11.

## Etapa 1: eliminación de los dos usuarios

PR #382 (`18ecda24506871112c170ebdcfada087b1e4ca22`) fue fusionado por `HernanEspinozaDev`, merge `e6c1d5d3f5aac2c05ca7a1ae3022577f2a072c47`. Se aplicó exactamente el plan aprobado SHA-256 `e6a04d3e301dcf82db3d82e2d8430637e71eb4e2d7793680ce2a3ef1a4ffa166`: 0 altas, 0 cambios y 2 eliminaciones.

Cloud SQL Admin API confirmó las dos operaciones `DELETE_USER` como `DONE`, ambas con `error=null`. La API ya no lista los usuarios y el state serial 12 no contiene recursos `google_sql_user`. La instancia sigue RUNNABLE con protección nativa habilitada; los seis Jobs siguen presentes y protegidos; el secreto y versión 1 continúan presentes/habilitados. Los cuatro objetos y generaciones siguen presentes según una lectura posterior de todas las versiones del prefijo. La evidencia saneada está en [iam-user-closeout-stage1-apply.json](evidence/gcp03-restore-g03r-1016/iam-user-closeout-stage1-apply.json).

La etapa 1 usó únicamente Cloud SQL Delete API a través del provider; no se ejecutaron sentencias SQL ni Jobs, y no se hizo un segundo intento. No hubo errores ni resultados inciertos.

### Corrección de evidencia stage0

La evidencia stage0 publicada anteriormente contenía un typo en `verify.id`: omitía `@espacigo-testprod.iam//`. En este PR se corrige el valor a `g03-verify-g03r-1016@espacigo-testprod.iam//espacigo-gcp03-restore-g03r-1016-pg` y se registra explícitamente el valor anterior y el corregido en el campo `corrections`. El state y plan reales siempre tuvieron el ID corregido.

## Etapa 2: plan completo de protecciones — pendiente de revisión

El plan se generó tras stage1, desde el `main` actualizado al merge de PR #382, contra state lineage `5bc6ad38-035f-fba3-2bcd-43bdf641e370`, serial 12. Es un plan completo, sin `-target`; Terraform muestra `complete=true`. Inputs relevantes: `enable_restore_iam_users=false`, `allow_restore_iam_user_destroy=true`, `allow_restore_destroy=true`, `confirm_destroy_run_id="g03r-1016"`, `enable_restore_jobs=true`.

El binario privado tiene SHA-256 `e89e9c70ee79ca1b5b88a025d1a2c711fcece2b3485957cc9cd5fac3bf9893ff`. Resultado: **0 altas, 9 cambios, 0 eliminaciones**. El cambio se limita a:

- Cloud SQL instance: `deletion_protection=true→false`, `deletion_policy=PREVENT→DELETE` y `settings.deletion_protection_enabled=true→false`.
- Base `espacigo_scratch`: `deletion_policy=PREVENT→DELETE`.
- Seis Cloud Run Jobs: `deletion_protection=true→false`.
- Secret metadata: `deletion_protection=true→false` y `deletion_policy=PREVENT→DELETE`; el secreto y la versión se mantienen.

No se recrean usuarios SQL IAM: `enable_restore_iam_users=false`, el state no contiene esos recursos y el plan no los crea ni los modifica. El validador [gcp03_run_protection_plan_validator.py](../../scripts/gcp03_run_protection_plan_validator.py) aceptó el JSON real y exige el conjunto exacto de recursos/campos, sin altas ni eliminaciones. Sus siete pruebas locales pasan. Evidencia saneada: [run-protection-stage2-plan.json](evidence/gcp03-restore-g03r-1016/run-protection-stage2-plan.json).

**Stage2 está pendiente de revisión y no se aplicó.** Las protecciones siguen activas hasta una eventual aplicación expresamente revisada. El futuro plan para eliminar recursos restantes es otra fase separada y no se ha generado.

## Conservación y límites

Permanecen fuera de alcance `pilot01`, el dataset persistente de origen y sus backups/archivos, red/subred basales, recursos compartidos y backend Terraform. Se mantienen retenidos:

| Objeto | Generación |
| --- | ---: |
| `runs/g03r-1016/manifest.json` | `1792195760678849` |
| `runs/g03r-1016/synthetic-v1-v61.dump` | `1792195769229365` |
| `runs/g03r-1016/dump.toc` | `1792195778303427` |
| `runs/g03r-1016/allowlist.toc` | `1792195786699241` |

No se envió borrado de objetos ni se borró secreto. Un futuro borrado de objetos requerirá autorización explícita y precondiciones `generation` e `ifGenerationMatch` iguales. Bajar protecciones no elimina recursos ni datos.

El hallazgo GCP-04 de `membershipCount==1` frente al diagnóstico `membership_count=2` no se relajó ni se alteraron memberships desconocidas. La discrepancia de `lastAttemptResult.status={}` de Cloud Run y los códigos de salida en logs sigue registrada como seguimiento acotado; no amplía aceptación. La aceptación de producto GCP-03/GCP-04 sigue pendiente.

No se ejecutaron pruebas cloud en esta etapa de preparación. Las lecturas de verificación fueron APIs de Cloud SQL Admin, Secret Manager, Cloud Storage y Cloud Run. No hay autosuspend configurado; el presupuesto/crédito restante no se verificó.
