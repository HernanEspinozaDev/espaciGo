# GCP-03: investigación de retire y diagnóstico de solo lectura — 2026-10-17

## Resultado de la investigación

La única ejecución previa de `retire` sigue sin ser un retiro exitoso. El detalle leído de Cloud Run para `espacigo-gcp03-restore-g03r-1016-retire-vlzxq` reporta `Completed=True` y `succeededCount=1`. La tarea tiene `lastAttemptResult.status={}` y no expone `exitCode` ni `termSignal`. Los logs de sistema, filtrados por contenedor, sí registran `retire` con salida `1` y `cloud-sql-proxy` con salida `0`. El log estructurado de la aplicación conserva `PHASE_FAILED`. Estos datos son incompatibles entre sí y no permiten explicar por qué el agregado de Cloud Run marcó la tarea exitosa; la causa de esa agregación queda abierta.

El código del ejecutor llama `os.Exit(execute())`; el flujo de error vuelve con código `1` después de solicitar el cierre del proxy. La nueva prueba invoca ese mismo flujo y comprueba que el error de fase no desaparece cuando el proxy cierra correctamente. También se ejecutó el binario del digest candidato en una instancia local sin red ni configuración y devolvió exit code `1` con JSON de falla saneado. Esto no reproduce la agregación del servicio Cloud Run.

La [API de tareas de Cloud Run](https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.jobs.executions.tasks) define `TaskAttemptResult.status`, `exitCode` y `termSignal`; indica que `exitCode` puede no estar presente cuando el contenedor no pudo terminar limpiamente con código. La documentación de [Cloud Run Jobs](https://cloud.google.com/run/docs/create-jobs) define éxito de ejecución en términos de que las tareas terminen con éxito. En este caso la respuesta de tarea no incluye un resultado que concilie el `exit(1)` observado en el log de `retire` con el agregado exitoso.

Se agregó [`scripts/gcp03_job_evidence.py`](../../scripts/gcp03_job_evidence.py) para requerir ambas señales: tarea Cloud Run exitosa **y** exactamente una evidencia de éxito de aplicación que coincida con fase y run ID. El validador probó la evidencia real anterior y rechazó `Completed=True` + `succeededCount=1` porque la aplicación reportó `PHASE_FAILED`. No se imprimen payloads crudos en su resumen.

## Cambios del ejecutor

- Las fallas de `retire` ahora usan formato `espacigo-gcp03-managed-synthetic-restore-failure-v2` e incluyen subpaso tipado para identidad, inventario e historial de migraciones, ACL previo, conteos, memberships, `DELETE` fijo, cada `REVOKE`, ACL/memberships posteriores y `COMMIT`.
- Se propaga el SQLSTATE de PostgreSQL cuando lo entrega el controlador. El detalle o mensaje SQL no entra en evidencia.
- El valor booleano previo `commit_outcome_uncertain=false` dejó de emitirse: el resultado se omite salvo que falle `Commit`, donde se reporta explícitamente `uncertain`. La evidencia de retiro exitoso declara `committed`.
- Cada JSON de falla declara `application_exit_code=1`, consistente con el código de salida del proceso.
- El diagnóstico de estado usa una transacción `REPEATABLE READ READ ONLY`. Devuelve conteos, estado de historial, flags de capacidad/membership para los dos usuarios IAM del run y coincidencia de contratos ACL; no devuelve nombres de cuentas de aplicación, datos de filas, mensajes SQL ni secretos.

## Job diagnóstico preparado para revisión

Se publicó la imagen inmutable para `linux/amd64`:

`southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03restore@sha256:f939c783628e633ffeb98a7b0487e9eaa27b0e3c91b3e87951482cc09dfbb85e`

La copia de lectura de Artifact Registry confirmó el digest. Terraform produjo el plan privado de solo presentación con SHA-256 `dcafb7f89987dd6cb176d7220331742c1b0a5725e2164158ffb44bde334a8f70`; el resumen hash-bound está en [retire-observability-plan.json](evidence/gcp03-restore-g03r-1016/retire-observability-plan.json). El plan crea solo `google_cloud_run_v2_job.retirement_diagnose[0]` y su binding de invocación `roles/run.invoker` limitado a ese Job: **2 creates, 0 updates, 0 deletes**. El Job reutiliza el service account admin y la versión numérica del secreto ya autorizados para `g03r-1016`, se fija a la base `espacigo_scratch`, no tiene retries y no cambia Jobs, SQL users, IAM existente, instancia ni bucket.

El plan binario y sus variables permanecen bajo `.local/testing/gcp03-1017/`, modo `0600`; Terraform no lo aplicó y el Job no se invocó. El plan se generó contra el backend state existente y solo refrescó metadata de infraestructura. No se ejecutó SQL contra Cloud SQL en este seguimiento. La identidad de base, conteos, memberships y ACL reales permanecen desconocidos hasta que se revise y libere el Job de diagnóstico.

## Ensayo local y corrección de fixture

`bash scripts/test-gcp03-restore-executor-postgres.sh` pasó en PostgreSQL 18/PostGIS con roles sin superusuario, ownership de objetos y los checks de retire antes/después. El diagnóstico mostró `transaction_read_only=true`, conteos `2/1/2`, memberships esperadas y ACL de contrato antes y después; la prueba no tocó GCP.

Se corrigió la descripción de fixture vinculada a PR #378: son dos usuarios sintéticos de aplicación, un espacio y dos filas de ocupación. `postgres` es el administrador SQL separado, no un tercer usuario de la aplicación. Se actualizó el hash SHA-256 en la referencia versionada al JSON de retire.

## Límites y siguiente paso

No se reintentó retire, no se ejecutaron restore/verify, no se aplicó Terraform, no se invocó el diagnóstico y no se eliminaron usuarios, objetos, secretos, Jobs, cuentas, base o instancia. GCP-03 sigue parcial. El siguiente paso depende de revisión del PR y liberación del plan/Job exactos; entonces se podrá ejecutar solo el diagnóstico de solo lectura y publicar sus conteos/ACL saneados. No se autoriza retire, teardown, importación de fuente ni GCP-04.
