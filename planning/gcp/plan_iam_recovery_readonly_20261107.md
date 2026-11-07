# Propuesta IAM temporal mínima — inspección recovery READ ONLY (2026-11-07)

## Decisión

**No aplicar IAM hasta probar y revisar el delta y su integración.** El bloqueo de listado amplio descrito en la propuesta inicial se aborda mediante la resolución parental V2 del final: no usar el comando v1 para el provisioner. La secuencia determinista requiere una pausa explícita del parent después de crear y verificar el request inmutable, pero antes de `builds submit` del provisioner. El operador concede lectura solamente al objeto request exacto ya conocido, confirma por lectura IAM y deja un recibo validado; recién entonces el parent continúa una sola vez. No conceder `recovery-inspections/` ni otro prefijo. No modificar el lock, sus verificaciones, el control de no-retry ni ejecutar el Job aplicador.

Este documento propone el delta; no aplica permisos ni modifica código. Identidad: `espacigo-release-job-builder@espacigo-testprod.iam.gserviceaccount.com`. El preflight dice que sus bindings inspeccionados no prueban denegación efectiva global (hay herencia organizacional desconocida), que Troubleshooter devolvió `SERVICE_DISABLED`, y que no se habilitó. Mantener esa API deshabilitada.

## Secuencia propuesta y orden obligatorio

1. Un parent dedicado, manual y fijado al SHA/archive ya revisado, valida su identidad/fuente, lock retenido y fallo histórico; construye/verifica la imagen como ahora. No iniciar rollout normal.
2. Parent genera el `build_id` Cloud Build asignado, escribe `gs://espacigo-testprod-cloud-build-447373903153/recovery-inspections/<PARENT_BUILD_ID>/job-request.json` con create-if-absent, verifica generation/hash por readback y conserva sus recibos existentes.
3. **Pausa fail-closed nueva:** no enviar el build del provisioner hasta leer un recibo operador íntegro que identifique exactamente principal, parent ID, URI/generation/SHA request, lock generation, Job histórico, roles/condiciones/expiración previstas y confirmación de readback IAM. Rechazar recibo ausente, expirado, duplicado, malformado o que no coincida; no reintentar ni resometer automáticamente el build. Esta pausa es necesaria: el ID no existe antes de crear el parent y el objeto request aún no existe antes del paso 2. No usar un grant de namespace como solución de carrera.
4. Operador toma snapshot de políticas y etags actuales; agrega bindings temporales mínimos; lee de vuelta políticas y verifica principal, role, condición exacta y expiración. Emite el recibo anterior solo después de esa verificación. Nada en el flujo actual demuestra que exista ya este gate/recibo: es un requisito de implementación del parent, no una capacidad declarada existente.
5. Parent valida el recibo y remite **una sola vez** el provisioner. Mantiene las validaciones y comprobaciones de lock existentes. Si IAM, recibo, build o resultado es incierto, detener y preservar estado/evidencia; no repetir el submit ni `execute`.
6. Tras terminar la inspección y la limpieza propia ya prevista, revocar únicamente los bindings temporales identificados, usando etags actuales; readback de políticas y comprobación de ausencia del principal/role/condición temporal exactos. Comparar contra baseline y demostrar que bindings preexistentes permanecen idénticos. Conservar snapshots, etags, recibo de grant/readback y recibo de revocación/readback.

## Delta candidato de mínimo privilegio

### Cloud Storage — exact object names

Agregar temporalmente `storage.objects.get` mediante un **custom role nuevo, task-owned**, por ejemplo `espacigoRecoveryReadonlyInspectionStorage20261107`; no editar `espacigoReleaseJobProvisioner` ni `roles/logging.logWriter`. Bind en la política IAM del bucket `espacigo-testprod-cloud-build-447373903153`, con CEL que enumere igualdad exacta de `resource.name` para:

- `projects/_/buckets/espacigo-testprod-cloud-build-447373903153/objects/cloudrun-main.lock`
- `projects/_/buckets/espacigo-testprod-cloud-build-447373903153/objects/recovery-inspections/<PARENT_BUILD_ID>/job-request.json`

Añadir `request.time < timestamp('<EXPIRY_UTC>')`. El segundo nombre solo se fija después del paso 2. No conceder `storage.objects.list`, `create`, `update`, `delete`, ni lectura de `releases/` o de todo `recovery-inspections/`. Preservar los bindings actuales `source/` y `releases/` byte/semánticamente; no reemplazar una política completa con snapshot viejo.

### Cloud Run — bloqueo técnico antes de IAM

El preflight necesita `run.executions.get` y `run.executions.list` únicamente para Job fallido `espacigo-db-r-2dc002a95831471780646432b98d3e01` y la ejecución retenida `...-v62zb`; no necesita `run.jobs.run`. La documentación oficial confirma que esos permisos existen y enumera `run.jobs.get`, `run.jobs.create`, `run.jobs.delete`, etc. por separado. Sin embargo, **no se confirma con las fuentes consultadas** que `resource.name` condition o un binding a la política IAM del Job pueda autorizar `run.executions.list` de forma restringida al Job/ejecución concreta, ni el formato de nombre de recurso que evaluaría cada llamada. El método REST `jobs.setIamPolicy` solo demuestra que hay política IAM en el recurso Job; no demuestra alcance efectivo de permisos de executions/list.

Por tanto, no inventar el CEL ni adjuntar un role amplio. Antes de cualquier IAM apply, obtener confirmación primaria oficial explícita de que ambas llamadas aceptan condición `resource.name` con nombres exactos y que la lista queda limitada, o una prueba controlada no productiva que demuestre ese alcance. Si `run.executions.list` solo se concede a nivel proyecto o no admite la condición exacta, **bloqueo**: no aplicar este delta, no ampliar a todas las ejecuciones/proyecto. Resolver con diseño de lectura que elimine `list` y use una referencia de ejecución inequívoca solo si el código/evidencia puede mantenerse seguro sin enumeración; de lo contrario, solicitar una decisión humana sobre un mecanismo alternativo de privilegio mínimo. No tocar el Job ni IAM del Job hasta resolverlo.

Si se demuestra soporte, crear rol nuevo, por ejemplo `espacigoRecoveryReadonlyInspectionExecution20261107`, con únicamente `run.executions.get` y `run.executions.list`; binding condicionado a Job/ejecución exactos y expiración, en el menor recurso donde la documentación garantice evaluación correcta. No añadir `run.jobs.run`, `run.jobs.update`, `run.jobs.setIamPolicy`, `run.executions.cancel/delete`, `run.tasks.*` o permisos de secretos/SQL. Si el único binding técnicamente válido es a nivel proyecto con alcance no demostrablemente exacto, queda rechazado.

### Permisos preexistentes

No cambiar el custom role `espacigoReleaseJobProvisioner`, `roles/logging.logWriter`, su binding, ni los grants de bucket `source/`/`releases/`. El job creator conserva solo sus permisos operativos ya existentes y en esta propuesta recibe únicamente lectura temporal adicional. No añadir permisos SQL, payload de secreto, API enablement ni impersonación.

## Expiración, etags, baseline y retirada

- Usar expiración IAM Condition `request.time < timestamp(...)` en cada binding temporal y `title`/`description` identificables. Expiración finita aprobada antes del apply; recomendación: ventana de hasta 2 horas desde grant, suficiente para la inspección, sin renovación automática. Eliminar los bindings tan pronto termine la operación; expiración es defensa adicional, no reemplazo de revocación.
- Guardar, antes de escribir, respuestas completas sanitizadas de `getIamPolicy` con versión y etag para bucket, proyecto y/o Job según el alcance finalmente probado; inventario exacto de bindings relevantes, custom role definitions previas y hora UTC. No registrar secretos ni tokens.
- Cada escritura usa el etag vigente (precondición); en conflicto, releer y reconciliar, nunca sobrescribir ni reintentar ciegamente. Después de cada escritura, `getIamPolicy` y comprobar binding exacto. Crear roles con ID task-owned irrepetible; no mutar roles preexistentes.
- Retirada: remover solo el binding temporal agregado, con etag leído inmediatamente antes; leer de vuelta. No restaurar la política completa desde baseline (podría borrar cambios concurrentes). Verificar roles preexistentes y bindings preexistentes sin cambios; si el role temporal queda sin bindings, borrarlo solo tras readback de ausencia y si no hay consumidores. Evidencia final: IDs de policy, etags pre/post, expresión/expiry, principal, permisos, lecturas exactas, diff semántico baseline→post-grant→post-revoke y ausencia del binding. La expiración por sí sola puede dejar binding en política; comprobar y retirar explícitamente.

## Hechos documentados y límites

- **Documentado:** IAM Conditions admite `resource.name` para Cloud Storage Objects con formato `projects/_/buckets/.../objects/...`; el nombre debe incluir la ruta completa. Storage documenta condiciones por nombre de objeto y expiración temporal, y que `storage.objects.list` no puede restringirse a subconjunto por ese atributo. Fuente primaria: Google Cloud, *Resource attributes for IAM Conditions* y *Cloud Storage IAM*.
- **Documentado:** Cloud Run enumera `run.executions.get/list` separadamente de permisos de Jobs. Fuente primaria: Google Cloud, *Cloud Run IAM permissions*.
- **Documentado:** existe endpoint REST `projects.locations.jobs.setIamPolicy`. Su existencia no prueba que condicione la enumeración de ejecuciones.
- **No verificado / no asumir:** soporte `resource.name` para ejecuciones Cloud Run en estas llamadas, formato evaluado, ámbito efectivo de `list`, binding Job-level suficiente. Es el blocker de aprobación/apply.
- **Observado en evidencia local (no prueba de acceso efectivo):** bucket y roles actuales; ausencia de binding directo del provisioner al Job histórico; Troubleshooter deshabilitado; herencia/grupos no descartados.

Fuentes oficiales (consultadas 2026-11-07):
- https://docs.cloud.google.com/iam/docs/conditions-resource-attributes
- https://docs.cloud.google.com/storage/docs/access-control/iam
- https://docs.cloud.google.com/run/docs/reference/iam/permissions
- https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.jobs/setIamPolicy

## Resolución parental de alcance y delta final en implementación

La inspección del SDK instalado confirmó un problema concreto: `surface/run/jobs/executions/list.py:123–138` llama a `ListExecutions(namespace_ref, label_selector)`; `--job` es un filtro de etiquetas v1, no convierte la autorización en un listado por recurso Job. No se concede el permiso amplio para acomodar esa llamada.

La documentación REST V2 primaria especifica autorización `run.executions.list` sobre el **parent Job indicado** y `run.executions.get` sobre el nombre de ejecución indicado. Se verificaron como operador ambas lecturas **GET**, sin SQL, con rutas V2 exactas, nunca `jobs/-`, y proyección de campos de estado/identidad. Resultado real: una ejecución `...-v62zb`, UID `3c86092e-476c-4fd4-9377-a57d87acdace`, `job` como identificador corto, `Completed.state=CONDITION_FAILED`, `failedCount=1`; counters ausentes se interpretan como cero. Esto prueba endpoint/contrato para el operador, **no** autoridad del provisioner.

Se implementa un cliente GET V2 aislado con credencial SDK capturada solo en memoria, sin impersonación/payload de Secret Manager, sin redirects del bearer, con URLs/field masks fijados. El padre debe integrar su respuesta sin retirar las comprobaciones de intención, UID del Job, ejecución única fallida y generación/texto del lock.

Para no depender de condiciones de política de Job no demostradas ni conceder un rol amplio: **un custom role nuevo y propio** `espacigoRecoveryRead20261107<8-hex>` con exactamente `storage.objects.get`, `run.executions.get`, `run.executions.list`; **un binding temporal en la política de proyecto**, `version=3`, con igualdad CEL de `resource.name` únicamente para los dos objetos Storage y el Job/ejecución históricos (formas exactas con project ID y project number), combinada con expiración ≤2 h. No hay prefijos ni wildcard ni `run.jobs.run/update`, cancel/delete, secrets o SQL. Un atributo no disponible o nombre diferente no satisface las igualdades: cualquier denegación se detiene y revoca, no se amplía la condición. La efectividad positiva se verificará mediante el provisioner real; el preflight de metadata no se rebautiza como ese ensayo.

El binding se calcula solamente cuando se conoce el parent ID real y su request inmutable. La nueva barrera `recovery_iam_gate.py` espera un recibo `iam-ready.json` del **mismo parent**, verifica source SHA, URI/generation/SHA del request, principal, binding exacto, expiración y hashes/etag de readback operador; revalida el lock durante la espera. Ausencia permite solamente polling de lectura acotado; recibo presente inválido aborta. **4 tests unitarios PASS** de esa barrera aislada; aún no se presenta como integración de parent publicada. El operador solo publica el recibo create-only tras leer de vuelta custom role y política aplicada. No se toca una política desde un snapshot viejo: CAS por etag y diff semántico, y revocación usando política actual quitando exclusivamente el binding propio.

Fuentes que cierran el ámbito HTTP/autorización de las llamadas:
- https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.jobs.executions/list
- https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.jobs.executions/get

Estado: **sin IAM apply, sin rol nuevo, sin build/Job de inspección nuevo**. Falta aceptar las implementaciones delegadas, integrar, probar y revisar el candidato antes de operar el delta autorizado. La consulta amplia de permisos testables se agotó en timeout sin mutación; los permisos ya están presentes en custom roles observados, pero eso no concede autoridad a esta identidad ni sustituye el readback del delta.

## Integración parental y candidato congelado para revisión

Se integraron el lector V2 de metadata y la barrera antes del único submit delegado. Se conservan intención/UID del Job histórico, lista única sin página siguiente, UID igual entre list/get, estado terminal fallido y lock exacto. Las lecturas reales del operador mediante el código integrado devolvieron el fallo histórico y la misma generación del lock, sin SQL ni ejecución nueva.

La aceptación parental de helpers detectó y corrigió antes de cualquier IAM apply: `resource.name` relativo en vez de full-resource URI con `//service`, operador CEL `||` en vez de `or`, rechazo de una condición que conservaba los seis fragmentos permitidos pero añadía `|| true` mediante comparación canónica completa, y validación de ventana de expiración al conceder. Se reprodujeron los negativos en RED. El lector también rechaza una URL ajena antes de obtener credencial y comprueba el handler real que bloquea redirects.

`recovery_iam_operator.py` implementa grant/revoke separados de la clase cloud de observación; guarda baseline y lease privados antes de escribir, revalida fuente/parent/lock, usa rol nuevo con tres permisos, CAS por etag, readback completo antes de readiness, y retirada desde política actual quitando solo el binding exacto. Las pruebas del operador usan IAM sintético; no se presentan como grants reales. Resultado parental completo: **121 tests Python PASS**; extracción solo versionada del árbol preparado **27e53c78a1200c48a506ea7a315ab81646aa6aa4** pasa además Go CLI/migrator/CloudSQL y todos los Python sin archivos privados/ignorados. `git diff --check` PASS.

Producción congelada: **11 hashes** en `/home/nandev/.hermes/cache/scratch/recovery-iam-review-manifest-20261107.json`, revalidados sin drift. Revisión independiente de operador/policy y lector/gate/wiring en `deleg_ba1dc30a`, pendiente de recibir y adjudicar. Los helpers/grants aún no se publican ni ejecutan sobre IAM hasta cerrar ese gate. Los nombres/permisos de rol, estado SQL y autoridad efectiva del provisioner no se infieren de los tests sintéticos.

Regresión parental adicional **RED** posterior a los 121 PASS: `test_create_collision_does_not_delete_an_unowned_role` reproduce una carrera entre el GET de ausencia y CREATE. Si otro creador obtiene ese ID y CREATE devuelve ALREADY_EXISTS, la retirada actual puede borrar por nombre un rol sin recibo propio de creación. **Bloquea cualquier IAM apply**; no se acepta el nonce como prueba de ownership. Se preservó el candidato congelado mientras termina su revisión; la corrección debe exigir recibo/readback de creación propio (y rechazar drift) antes de borrar ese rol, manteniendo la retirada del binding exacto por separado. El fallo proviene únicamente de IAM sintético; ningún rol real fue creado/borrado.

### Corrección/revisión IAM — ciclo 1

La revisión `deleg_ba1dc30a` aprobó lector/gate/wiring y objetó el operador. La adjudicación separó garantías reales de hipótesis: setIamPolicy ya recibía el etag actual y el servidor aplica CAS; no era necesario demostrar que el etag cambiara por sí solo. Se añadió además comprobación de la respuesta de escritura y del GET separado. Retirar solo el binding propio desde la política actual **debe preservar** cambios concurrentes ajenos: no se restaura baseline ni se deja el grant activo por un cambio no relacionado; se informa `baseline_preserved=false`/`concurrent_policy_changed=true` sin fingir restauración global.

Correcciones concretas: prueba de creación mediante respuesta CREATE guardada y GET de la misma revisión/etag antes de borrar; DELETE propio con etag usando REST porque el SDK instalado no expone ese flag; condiciones/roles/members de toda la política validados antes de crear rol; intención local de publicación escrita antes de readiness, sin escritura local posterior; ante handoff incierto no se retira automáticamente una autorización que el parent pudo consumir, se inspeccionan parent/objeto y se revoca al terminar; cada intento explícito de revocación genera recibo único ligado al readback actual, sin supresión por un archivo viejo.

Los negativos RED de colisión, handoff, recibo obsoleto, drift concurrente, política malformada y DELETE guardado quedan en **GREEN**. Suite completa: **127 tests Python PASS**. Metadata real del operador: proyecto devuelve política v3 íntegra, **33 bindings**, sin transformación de condiciones; testIamPermissions confirmó los **7 permisos administrativos/de build solicitados**, sin mutación; una consulta a nombre de rol nonce confirmó clasificación SDK NOT_FOUND sin crearlo. Esto no prueba autoridad efectiva del provisioner ni un grant ejecutado.

Nuevo congelado de **11 hashes**: `/home/nandev/.hermes/cache/scratch/recovery-iam-review-manifest-cycle1-20261107.json`. Solo cambiaron operador/policy respecto al congelado anterior; los otros **9 archivos** permanecen idénticos, por lo que sigue vigente la aprobación del lector/gate/wiring. Revisión de correcciones `deleg_1ecd0bdd` pendiente. **Sigue sin existir rol/grant nuevo ni build/Job de inspección remoto nuevo**; no operar IAM hasta adjudicarla.

Referencia primaria del DELETE con etag: https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.roles/delete . La expiración temporal es defensa adicional; la retirada explícita y sus lecturas posteriores siguen siendo obligatorias.

### Gate de fuente cerrado; operación autorizada aún pendiente

`deleg_1ecd0bdd` aprobó el ciclo 1 de operador/policy sin hallazgos; sus dos hashes coinciden y siguen coincidiendo los otros nueve archivos ya aprobados. Recibo: `evidence_iam_recovery_source_review_20261107.json`. La extracción únicamente versionada actualizada del árbol **aa0092acec452eb06e5c69db5db563bcdf775149** vuelve a pasar los **127 Python** y Go focalizado. Ningún resultado simulado se contabiliza como IAM real o ledger Cloud SQL.

Con esa revisión se habilita publicar solo en la rama de inspección y preparar el único parent manual del SHA publicado. Esperar su request exacto antes de conceder IAM; emitir readiness solo tras readback. Conservar lock histórico; no reintentar `execute`. Retirar el binding/rol propios y confirmar por lecturas al finalizar, incluso si el parent falla. Ante handoff incierto, comprobar parent y objeto immutable antes de retirar. Un fallo de permisos no autoriza ampliar selectors o permisos fuera del delta revisado.
