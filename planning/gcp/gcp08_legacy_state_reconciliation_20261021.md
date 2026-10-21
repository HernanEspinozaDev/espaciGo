# GCP-08 — reconciliación de bindings SQL legacy del state (2026-10-21)

## Objetivo y límites

Este corte prepara una operación manual, separada del preview y del apply de Cloud Run, para retirar del state canónico únicamente estos dos bindings administrados:

- `google_sql_database_instance.postgres[0]`
- `google_sql_database.application[0]`

`terraform state rm` elimina esos bindings de Terraform; no llama a Cloud SQL ni borra instancias o bases. No se tocarán otros addresses, incluida la data source `data.google_sql_database_instance.gcp06_candidate[0]`. La inspección debe confirmar que la instancia guardada es exactamente `espacigo-testprod-pg`, proyecto `espacigo-testprod`, y que la base `espacigo` apunta a esa misma instancia y no a `espacigo-testprod-db`. Cualquier discrepancia cancela la operación.

La vista previa protegida #39674916202 observó un `legacy_postgres_managed=1`, `legacy_database_managed=1`, `candidate_instance_data=1` y `v67_instance_data=0`. Una consulta efectuada por el propietario el 2026-10-21 informó HTTP 404 para el GET exacto de `espacigo-testprod-pg`; el GET con la identidad WIF restringida puede devolver 403 por su condición IAM y no sirve como evidencia de inexistencia. El inspector del state-remove repetirá la comparación de atributos privada antes de cualquier escritura.

No se ejecutó ni se autoriza aquí una escritura de state. El PR añade una operación manual protegida que requiere revisión y una corrida explícita después de fusionarse. La operación no está conectada al job de preview/apply y no lanza un preview automáticamente.

## Flujo de inspección y retiro

La operación usa el workflow existente `Application rollout`, el mismo concurrency group `testprod-cloud-run-rollout`, la misma raíz GCS y el workspace `default`. El WIF está limitado al workflow exacto en `main`, evento `workflow_dispatch`, repositorio/IDs fijos, actor GitHub inmutable `176093371` y entorno protegido `testprod`. El job vuelve a comprobar esos valores y falla cerrado ante una diferencia. No se añaden permisos IAM ni se usa Owner/Editor.

1. Después de fusionar y revisar el PR, lanzar `operation=legacy-state-reconcile`, `mode=state-inspect` desde `main`. El inspector usa `terraform workspace show` y `terraform state pull` con salida y state solo bajo `$RUNNER_TEMP` (directorio `0700`, archivos `0600`). El resumen contiene el serial y booleanos/cantidades de aliases fijos; no contiene IDs, nombres arbitrarios, correos ni valores del state.
2. Revisar el resumen: cada binding debe existir exactamente una vez, el nombre/proyecto de la instancia deben coincidir con legacy y la base debe estar ligada a legacy, con `database_is_v67=false`. Registrar el serial resultante para la operación siguiente. Si no coincide, detenerse; no retirar nada.
3. Antes de `mode=state-remove`, confirmar otra vez el GET SQLAdmin 404 exacto bajo una credencial humana autorizada y seleccionar `legacy_instance_absence=owner-get-404`. No usar la respuesta WIF 403 como ausencia. El environment protegido requiere aprobación de revisión para esta corrida.
4. Proporcionar el serial inspeccionado y la frase exacta `REMOVE ONLY GCP08 LEGACY SQL STATE BINDINGS`. El job vuelve a leer el state y vuelve a comprobar el serial y los dos bindings. Si cambió serial, lineage, atributos o cualquier parte no objetivo del state, cancela antes de `state rm`. El fingerprint ignora solo `serial`, `check_results` (metadata de checks que Terraform 1.16 puede actualizar entre pulls sin mutar bindings) y las dos instancias legacy objetivo.
5. El job copia el objeto GCS canónico **server-side**, sin descargar ni mostrar el contenido, a `espacigo/testprod/gcp/manual-state-backups/gcp08-legacy-sql/<run-id>-<attempt>/default.tfstate`. El origen queda fijado por generación y el destino requiere generación cero. Lee metadatos privados del objeto original/copia y exige mismo tamaño, CRC32C y MD5. La copia permanece bajo el managed folder privado existente; no se crea un bucket ni se amplía IAM.
6. Después de verificar la copia y releer serial/lineage/atributos, el job ejecuta exclusivamente `terraform state rm -lock=true -lock-timeout=5m` con los dos addresses enumerados arriba. Terraform mantiene el lock remoto durante la escritura. No se usa `-lock=false`, `force-unlock`, edición JSON del state ni una ruta de apply.
7. Un pull posterior verifica que desaparecieron ambos bindings, que lineage permanece, que serial avanzó y que el hash lógico del state estable coincide con el snapshot previo, excluyendo `serial`, `check_results` y solo las dos instancias `[0]`. Un fallo de verificación conserva la copia en GCS y bloquea el paso siguiente; no intenta restaurar automáticamente un snapshot potencialmente obsoleto.
8. Solo tras la corrida state-remove aprobada, lanzar por separado el `initial-restore / preview` normal. El validador actual debe rechazar toda acción SQL. No ejecutar apply hasta revisar y aprobar el nuevo resumen/huella.

## Reversibilidad y recuperación

La copia de state queda en el bucket privado existente aun cuando el bucket no tenga versioning/retention. Su URI se deriva del `run_id` y `run_attempt` de la corrida; CRC32C/MD5/tamaño y generación se verifican en privado. No se publica metadata ni state como artifact de GitHub.

Si hay que recuperar, detener previews, applies y otras escrituras de state; abrir una nueva operación manual revisada. Confirmar que el state actual es exactamente el resultado posterior al retiro: lineage esperado, serial de la corrida de retiro, ambos bindings ausentes y state lógico restante idéntico al backup excluyendo `serial`, `check_results` y esos dos bindings. Descargar la copia solo para recuperación, con `umask 077` en un directorio privado; verificar sus metadatos/checksums; luego usar `terraform state push -force <backup-local>` desde la raíz canónica, que toma el lock remoto. `-force` es necesario porque el serial del backup es anterior y omite las protecciones automáticas de serial/lineage, por lo que requiere aprobación explícita del operador revisor. No usar `force-unlock`; no hacer push si hubo una escritura ajena o si el estado restante difiere. Volver a comprobar lineage, serial, aliases y el state restante después del push. La copia GCS se conserva.

El `state push -force` no forma parte del workflow de retiro ni corre como rollback automático. Si sus precondiciones no se cumplen, detener y preparar un procedimiento revisado para reconciliar el state actual.

## Validación del corte

Las pruebas mock cubren identidad/ref/evento/entorno, coincidencia exacta de instance/database, serial, filtro por instance index, backup GCS generation-pinned/create-only y comparación de metadatos, `state rm` exacto con lock y conservación de todos los demás recursos. No leen el state remoto ni hacen llamadas a GCP. El job `state-inspect` es la primera operación autorizada tras la fusión; `state-remove` sigue sujeto a los gates anteriores.
