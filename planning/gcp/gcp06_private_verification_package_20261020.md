# GCP-06: paquete local de verificación SQL privada

**Estado:** preparado localmente para revisión; no ejecutado en Cloud ni Cloud SQL.
**Base:** `main` en `54fda4e8199384ba2798df981d8f37399774dc50`.
**Objetivo fijo:** verificar la candidata importada V1–V64, sin repetir dump, transferencia ni importación.

## Separación de consultas y permisos

### Fase A — inspector catalog-only existente

`cmd/gcp06sqlinspect` / `internal/gcp06sqlinspect/inspect.go` permanece intacto. Su contrato usa una transacción PostgreSQL `READ ONLY`, destino fijo, consultas allowlistadas sobre `pg_catalog` y extensiones, y resumen de owners y privilegios efectivos. No consulta filas de `public.schema_migrations` ni de tablas de aplicación. El manifiesto de consultas asigna IDs `GCP06-CAT-001` a `GCP06-CAT-011` a sus símbolos SQL existentes y fija el SHA-256 del archivo fuente.

Fase A no confirma ledger ni conteos de datos. No suponer que el login temporal elevado tiene `SELECT` sobre esos objetos y no ampliar su inspector ni su Job para leerlos.

### Fase A2 — agregados fijos de filas, artefacto separado

`planning/gcp/sql_candidates/gcp06_candidate_data_verify_v1_v64.sql` contiene el único SQL adicional (`GCP06-ROWS-001`). Verifica localmente la base declarada, PostgreSQL 18 y `session_user = current_user`, sin `SET ROLE`; esas constantes **no prueban** el proyecto ni la instancia remotos. Inicia una transacción `READ ONLY` con timeout de diez segundos y emite un objeto JSON agregado con identidad de sesión y hora UTC observada.

Comprueba ledger V1–V64 (`rows`, mínimo, máximo, distintos, duplicados y continuidad) y compara exactamente el perfil `staged` que genera `scripts/gcp06-local-stage-export.sh`: `migration_count`, `migration_max`, `users`, `spaces`, `active_spaces`, `catalog_candidates`, `synthetic_kyc`, `bookings`, `active_users`, `admins`, `global_admins` y `disabled_hashes`. No enumera tablas o filas, ni selecciona columnas personales, UUID, hashes, sesiones o tokens. No agrega los privilegios SQL necesarios: si el actor autorizado no puede leer cualquiera de esos agregados, `psql` se detiene con error y no produce un resultado válido. No reintentar ni cambiar grants como respuesta al error.

Esta fase es independiente de Fase A y requiere su propia revisión del actor, canal, permisos efectivos y plan de ejecución/limpieza. El script no afirma que el login de `gcp06sqlinspect` pueda leer filas. La aprobación de este paquete, si se fusiona, tampoco autoriza la conexión ni la operación Cloud.

### Comparador y manifiesto

`planning/gcp/gcp06_candidate_verification_manifest_20261020.json` fija target, query IDs, SHA de código fuente y SQL, campos de salida y política de redacción. El comparador `scripts/gcp06_candidate_verify_manifest.py` exige tres archivos privados en modo `0600` bajo directorios `0700`: manifiesto, JSON de `GCP06-ROWS-001` y evidencia de destino obtenida por el canal aprobado (salida JSON proyectada de `gcloud sql instances describe`). Comprueba `name`, `region`, `connectionName`, `databaseVersion` y `state` contra el proyecto/instancia fijados, y valida que el resultado SQL registre `session_user = current_user` y hora UTC. Sin evidencia, o con un recurso distinto, devuelve `STOP`. Reporta hashes de manifiesto/evidencia más identidad de sesión y hora observada; no imprime conteos ni contenido de los archivos.

El manifiesto privado de staging no está en este worktree. Por eso `manifest_sha256` queda `null` en el manifiesto de consulta y el comparador lo calculará únicamente cuando el archivo privado esté disponible por el canal local autorizado. No se inventó el hash ni se copió contenido de `.local`. El hash/bytes del objeto importado fijados por la evidencia versionada son `bb35a8709ae74d51e73b69d0dd75d7b4b761822508353dab9dddcd631cec9c11` / `16620453`; esto identifica el dump comunicado, no demuestra su verificación SQL.

## Precondiciones y ejecución futura

Antes de ejecutar, una revisión separada debe identificar y autorizar el canal privado, actor SQL, recursos exactos, consulta `GCP06-ROWS-001`, coste y evidencia de rollback. Obtener mediante el canal aprobado la evidencia JSON proyectada del recurso exacto; comparar proyecto/instancia/conexión, región, motor y estado. La evidencia debe corresponder a la misma sesión de verificación; conservar hora y `session_user` informados por SQL y el hash del archivo de evidencia. La consulta no identifica por sí sola proyecto ni instancia y no debe aceptarse sin el archivo de evidencia. No reutilizar una credencial local ni asumir acceso del inspector. El actor SQL debe ser exactamente el login autorizado de esa fase, sin cambio de rol, con permisos de lectura efectivos para los objetos que cubren los agregados. La consulta está fijada a la candidata y fallará si falta una tabla/permisión.

Los archivos de ejecución permanecen bajo `.local/testing/` con directorios `0700` y archivos `0600`; no copiar DSN, contraseñas, filas o manifiestos al repositorio, terminal compartido, logs, capturas o PR. En la sesión privada, guardar la salida de `GCP06-ROWS-001` en un archivo privado de modo `0600`; después ejecutar localmente:

```bash
python3 scripts/gcp06_candidate_verify_manifest.py \
  --manifest .local/testing/gcp06-stage-<run-id>/manifest.json \
  --actual .local/testing/gcp06-verify-<run-id>/actual.json \
  --target-evidence .local/testing/gcp06-verify-<run-id>/target-evidence.json
```

`target-evidence.json` debe ser la salida saneada capturada del canal aprobado, por ejemplo `gcloud sql instances describe espacigo-gcp06-20261020-gcp06a-pg --project espacigo-testprod --format='json(name,region,connectionName,databaseVersion,state)'`. Esta instrucción es futura y no se ejecutó en este corte. La vinculación del informe resultante conserva `session_user`, `current_user` y `observed_at`; rechaza una evidencia ausente o que no coincida exactamente con el destino esperado.

Solo `PASS` habilita la revisión humana de los resultados del catálogo y los agregados. La evidencia registrará hash del manifiesto, hash del dump, hashes de consulta/código, IDs de consulta, actor aprobado sin divulgar identidad personal, destino, hora UTC, resultado y límites. No incluye filas ni secretos.

## Errores, rollback y limpieza futura

- Error de target o versión: el script imprime `GCP06_STOP_TARGET` y termina antes de los agregados. No aceptar el archivo; el comparador debe devolver `STOP` si la salida no es el objeto JSON exacto esperado.
- Error de conexión, timeout, tabla ausente, `permission denied` o forma de resultado inesperada: conservar solo el SQLSTATE y `GCP06-ROWS-001` saneados. Abortar la sesión; no cambiar rol, grants, propietario, password o configuración para reintentar.
- Error durante la consulta: cerrar la sesión o ejecutar `ROLLBACK`; la transacción fue `READ ONLY`, así que no hay cambios de datos que revertir. Retirar el archivo `actual.json` incompleto del directorio privado y no informarlo como evidencia. No repetir una consulta remota sin revisar el fallo.
- El query bundle no crea credenciales, Secret Manager versions, Jobs, IAM ni recursos; no requiere rollback Cloud. Si una futura operación autorizada elige el helper `gcp06-sql-access`, el comando exacto de preparación es `go run ./cmd/gcp06sqlaccess prepare --apply --confirm-target 'GCP06 SQL INSPECT PREPARE 20261020-gcp06a'`. Solo ejecutar tras revisión/aprobación del plan Cloud exacto; el helper exige el contenedor secreto exacto vacío con TTL de destrucción inmediata y crea una única versión/usuario temporal. Si el resultado es incierto, no repetir; reconciliar el login/versión indicados y el estado de operación.
- Tras completar el uso autorizado, el comando exacto de helper cleanup es `go run ./cmd/gcp06sqlaccess cleanup --apply --login '<login emitido por prepare>' --secret-version '<número emitido por prepare>' --confirm-target 'GCP06 SQL INSPECT CLEANUP 20261020-gcp06a <login emitido por prepare> <número emitido por prepare>'`. Debe verificarse `login_deleted=true` y `secret_version_destroyed=true`. El helper canonicaliza el número como `projects/447373903153/secrets/espacigo-test-gcp06-sql-inspector-credential/versions/<número>`, elimina y verifica primero el login SQL exacto, y solo entonces destruye/verifica esa versión exacta. No usar otro login, secreto, proyecto o versión.
- Si un plan futuro habilitó el Job de Phase A, retirar únicamente con plan Terraform separado y revisado los recursos run-scoped `google_cloud_run_v2_job.gcp06_sql_inspector[0]`, `google_cloud_run_v2_job_iam_member.gcp06_sql_inspector_invoker[0]`, `google_service_account.gcp06_job["sql-inspector"]`, `google_project_iam_member.gcp06_job_cloudsql_client["sql-inspector"]`, `google_secret_manager_secret_iam_member.gcp06_job_secret_access["sql-inspector-credential"]` y `google_artifact_registry_repository_iam_member.gcp06_job_image_reader["sql-inspector"]`. El plan no debe tocar la candidata, API/frontends, runtime, secretos compartidos, estado histórico o tráfico. El contenedor secreto debe quedar sin versiones; eliminarlo requeriría plan y aprobación separados. Nunca modificar `postgres`, API runtime, tráfico, base o datos para limpiar esta verificación.

## Hallazgo del nombre canónico de Secret Manager

El incidente previo del nombre de recurso que devuelve Secret Manager con número de proyecto ya está corregido en el helper existente. `CanonicalVersion("<número>")` lo canonicaliza solo al secreto fijo con proyecto `447373903153`; `validVersionName` acepta el nombre por ID de proyecto o número solo para ese secreto exacto. La prueba sintética existente `TestCanonicalSecretManagerProjectNumberNames` acepta el recurso canónico, extrae el número de versión y rechaza otro proyecto. Se validó por inspección de código y tests locales; no se hizo una llamada a Secret Manager ni se reintentó el helper.

## Pruebas locales realizadas

Todas son locales y sintéticas; el harness PostgreSQL usa imagen fijada, red Docker `none`, filesystem temporal, base descartable y filas ficticias.

- `python3 scripts/test_gcp06_candidate_verify_manifest.py`: PASS (7 casos; comparación, drift de ledger/conteo, permisos de archivo/directorio, evidencia de destino distinta y rechazo de campos personales).
- `scripts/test-gcp06-private-verification-postgres.sh`: PASS (consulta fija contra PostgreSQL 18 descartable, rechazo de DB ajena y denegación de lectura sin grants; no concede grants).
- `go test ./internal/gcp06sqlaccess ./cmd/gcp06sqlaccess ./internal/gcp06sqlinspect ./cmd/gcp06sqlinspect`: PASS; incluye el canonicalizador de Secret Manager y la garantía catalog-only del inspector.
- `bash -n scripts/test-gcp06-private-verification-postgres.sh`, `python3 -m py_compile ...` y `git diff --check`: PASS.

No se usaron credenciales, APIs Cloud, Cloud SQL, Secret Manager, Terraform, usuarios, datos reales ni recursos remotos. No se generó evidencia de verificación Cloud. GCP-06 y GCP-07 permanecen parciales/pendientes.
