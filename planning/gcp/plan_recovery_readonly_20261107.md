# Corte aislado — inspección privada de recuperación READ ONLY

## Autorización y separación

El usuario respondió **«Sí: publicar solo la ruta de inspección privada de recuperación»** a la autorización concreta: levantar la restricción anterior de commit/push/integración únicamente para un corte limpio DB/pipeline, separado de KYC, revisado y probado, que inspeccione ledger/catálogo actuales de `espacigo-testprod-db/espacigo_dms_stage_20261020` mediante Job privado. **No autoriza aplicar SQL, modificar permisos, borrar el lock ni desplegar la aplicación.**

- Base: `origin/main=5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`, comprobada por fetch y ls-remote.
- Rama: `codex/database/fix/recovery-readonly-inspection`.
- Worktree nuevo: `/home/nandev/Documentos/projects/espaciGo-db-recovery-readonly`.
- Worktree KYC/reconciliación dirty en `espaciGo-main-sync` permanece intacto. No trasladar sus cambios KYC/V85/policy a este corte.
- Autor/committer/Git/gh: HernanEspinozaDev; hooks `.githooks` habilitados. Sin PR ni force-push.

## Bloqueo real y objetivo acotado

El release `2dc002a9-5831-4717-8064-6432b98d3e01`, fuente `bb10309129adad9d7204b9b54bb2259186a34242`, conserva lock generación `1794036269394395`. Su ejecución aplicadora `espacigo-db-r-2dc002a95831471780646432b98d3e01-v62zb` falló el 2026-11-07 a las 07:38:31Z al aplicar V84: `permission denied for schema public (SQLSTATE 42501)`. La inspección canónica preapply rechazó desbloqueo porque existe intención aplicadora. No confundir ese fallo con ausencia demostrada de cambios de esquema: hace falta leer ledger/catálogo actuales.

Esta ruta añade un **modo observador**, no una recuperación mutante. No reutiliza el flag de aplicación, no relanza la ejecución fallida y no elimina el lock. La evidencia debe distinguir el ledger real de la base del inventario del observador V1–V84; no incluir la candidata V85 ni afirmar que el entorno cloud está corregido.

## Contrato del modo SQL

- Identidades fijadas: proyecto `espacigo-testprod`, región `southamerica-west1`, instancia `espacigo-testprod:southamerica-west1:espacigo-testprod-db`, base `espacigo_dms_stage_20261020`, login `espacigo_migrator_login`, `SET ROLE espacigo_schema_owner`, IP privada.
- Una transacción `REPEATABLE READ READ ONLY`, sin DDL/DML, locks de migración, workers ni filas de negocio.
- Leer ledger con nombres/checksums, versión de motor e identidades; reportar roles/memberships y hechos de autoridad/ACL/PUBLIC/defaults de dependencias acotadas. Un privilegio faltante se informa, no se concede.
- Credencial solo desde montaje numérico de secreto existente dentro del Job. No leer/descargar payloads desde el portátil o builder.
- Ningún flag de aplicación/ACL/bootstrap puede combinarse con el observador; rechazar argumentos posicionales y configuración inesperada antes de conectar.

## Ruta de publicación sin aplicación accidental

Publicar únicamente la rama de este corte. **No integrar/publicar main si activa la orquestación full normal**: esa acción excedería el permiso READ ONLY. Un build de recuperación dedicado ejecuta exclusivamente imagen migradora e inspección, sin construir/desplegar API/web, backup ni watermark. No se altera el gate/provisioner normal para aceptar padres arbitrarios.

La fuente debe ser el archivo del commit publicado exacto, no el worktree dirty. Verificar checksum/generación del archivo de fuente y provenance/digest de la imagen; una etiqueta `_COMMIT_SHA` no basta. Identidad del parent/child, imagen, parámetros, red privada, service account, secreto numérico, UID del Job y ejecución deben verificarse antes/después. La intención de inspección se conserva antes del único execute; no reintentar en resultado incierto.

Mantener evidencia propia por generación/hash bajo namespace separado `recovery-inspections/<build-id>`. Conservar lock original antes/después. Limpiar solo el nuevo Job READ ONLY de UID coincidente cuando el resultado esté persistido; conservarlo ante incertidumbre. No IAM, Terraform, cambios de cuentas ni actualizaciones de servicios.

## Gates de este corte

- [ ] Implementación SQL observadora: tests negativos de CLI/configuración y facts de catálogo; build/vet.
- [ ] PostgreSQL aislado real: snapshot de ledger/catálogo, transacción READ ONLY y ausencia de mutaciones; sin tocar base nativa persistente.
- [ ] Orquestación/provisioner: tests de rechazo temprano, parent/source/provenance, lock retenido, Job exacto, no apply/backup/rollout/unlock.
- [ ] Revisión parental independiente del diff completo y archivos nuevos; SQL V1–V84/policy/KYC sin cambios.
- [ ] Commit/push solo corte; identidad/hook y SHA remoto verificados; CI del SHA consultado.
- [ ] Build privado observador del SHA exacto, ejecución y resultado actuales comprobados, evidencias inmutables releídas.
- [ ] Registrar estado real de destinos y límites. Gate de **inspección** completado no equivale a gate DDL aprobado ni autoriza arreglar permisos/desbloquear/aplicar V85.

Estado al crear este plan: implementación/pruebas en curso; no hubo publicación ni build de recuperación ni nueva lectura SQL de Cloud SQL.

## Revisión de las entregas y continuación parental

La primera entrega Go no tenía ensayo PostgreSQL y fijaba por error `/secrets/admin/password` en vez del montaje existente `/var/secrets/migration/password`; la revisión solicitó corregir ese contrato y añadir ACL por columnas/owners y prueba de transacción real. El padre no acepta su unit PASS como evidencia de base.

La entrega inicial Python detenía siempre el flujo antes del Job y carecía de provisioner; no era publicable como ruta funcional. El padre completó la orquestación y provisioner separados, sin editar `cloudbuild.yaml`, release/provisioner normales, SQL, policy o KYC. Se preserva el lock histórico; los únicos efectos remotos previstos son builds de imagen/provision, nuevo Job observador y evidencia propia. El único execute se precede de recibo de intención; cualquier resultado incierto conserva recursos y no reintenta. La limpieza exige UID/configuración propios y confirma ausencia por listado exacto.

Validación parental ejecutada: `/usr/bin/python3 -m unittest discover -s scripts/gcp -p 'test_recovery_inspection*.py'`: **14 tests PASS**, incluyendo circuito padre con nube falsa, identidad/configuración exactas, fuente archivada, provenance de imagen, creación acotada, lectura inmutable y rechazo de aplicación/desbloqueo. Estos son tests offline, no Cloud SQL. `git diff --check` PASS; 2872 archivos tracked, sin symlinks. Sin diferencias en migraciones/policy/KYC/frontend/flujo normal respecto a la base. Identidad y hooks comprobados; no commit ni push todavía.

Pendientes antes de publicar/ejecutar: ensayo PostgreSQL real del inspector y revisión independiente de seguridad del código integrado, suite/regresión final y evidencia SHA/refs. No marcar esos gates por los resultados offline anteriores.

### Ensayo PostgreSQL parental y regresión posteriores

El siguiente subagente corrigió el montaje pero no implementó ACL por columnas ni ensayo real. El padre completó ambos. El ensayo comenzó en RED al faltar hechos separados USAGE del login/grupo; se añadieron hechos explícitos, owners/default owner y ACL de columnas con manejo de arrays vacíos. Se reporta también `transaction_isolation=repeatable read`, leído en la misma transacción READ ONLY que ledger/catálogo.

- `bash scripts/test-recovery-inspection-postgres.sh`: **PASS real, sin skip**, `TestRecoveryInspectionPostgresReadonlySnapshot`. Contenedor propio `espacigo-recovery-ci-1636281`, imagen PostGIS fijada, red/puertos/volúmenes ausentes, PGDATA tmpfs, socket privado. Verifica dos migraciones reales en fixture, roles/grants por columnas/PUBLIC/defaults, reporte de privilegios ausentes sin concederlos, identidad fijada y snapshot pre/post de ledger/catálogo igual. Caso ledger ausente no lo crea y devuelve las dos pendientes. Se retiró el contenedor y verificó ausencia.
- El primer contenedor RED se retiró, pero su socket heredó ownership del entrypoint y falló retirar el directorio scratch. Se corrigió limpieza del bind propio antes de eliminar contenedor; se retiró específicamente el directorio del ensayo RED tras restaurar ownership. No se tocó ningún volumen/base persistente.
- `go test -count=1 ./cmd/dbmigrate ./internal/migrator` y `go vet ./cmd/dbmigrate ./internal/migrator`: PASS (los opt-in DB de la suite ordinaria no equivalen al ensayo anterior).
- `/usr/bin/python3 -m unittest discover -s scripts/gcp -p 'test_*.py'`: **89 tests PASS**. ResourceWarning de fixtures HTTPError preexistentes; los mensajes simulados de release/schema no representan operaciones cloud.
- `git diff --check`: PASS. No publicación/Job remoto todavía; falta incorporar el resultado de revisión independiente de seguridad antes de commit/push/build.

### Revisión de seguridad y endurecimiento parental

La revisión independiente señaló comparación parcial del task, límites amplios de comandos, evidencia local sobreescribible, falta de intención de cleanup y ausencia de lectura del lock alrededor del create delegado. Se reprodujeron negativos antes de corregirlos y se acotó:

- Comparación completa del task de Job y ejecución contra un contrato, incluido nombre `inspection`, una sola imagen/contenedor/volumen, recursos, env, montaje y secreto versión `1`. Solo se normalizan orden de env, nombre generado de volumen, timeout numérico/string y `sandboxes={}` vacío. Campos adicionales se rechazan. La revisión exageró el caso sidecar/env extra: el código anterior ya comparaba su cantidad/lista; el defecto confirmado era permitir otros campos no revisados.
- Argv de creación exactamente derivado del contrato; builds solo con configuraciones `/workspace/` fijadas, SA correspondiente y sustituciones explícitas. La ayuda instalada de gcloud exige opciones de Job antes de `--container`; se corrigió el orden y se añadió regresión.
- Evidencia local `O_EXCL|O_NOFOLLOW`, directorios sin symlinks, fsync, creación remota condicionada a generación cero y readback. Un reinicio con recibos existentes falla: no se añade replay/resume automático del execute. Intención de cleanup inmutable antes de borrar, readback de UID/configuración propios y posterior ausencia.
- Lock fijado leído inmediatamente antes/después del create delegado. El reporte exige aislamiento `repeatable read`, identidad y hechos de autoridad con tipos completos, sin exigir que los privilegios observados sean correctos. Log unido a ejecución/UID exitosos, recurso/stdout y timestamp dentro de su ventana real.

Validación parental tras cambios: **16 tests recovery/security PASS**, Go focalizado y vet PASS, diff check PASS. Nuevo ensayo de contenedor propio `espacigo-recovery-ci-1653818`: **PostgreSQL real → JSON Go → validador Python de producción PASS**, sin skips, ausencia del contenedor confirmada. El primer intento de esa prueba puente falló porque aún no emitía el JSON; se mantuvo como RED y luego se añadió emisión solo en el test sintético. La prueba no abre Cloud SQL ni lee filas de negocio.

Candidato de producción congelado para nueva revisión independiente en `/home/nandev/.hermes/cache/scratch/recovery-review-manifest-20261107T184206.json`. Las pruebas adicionales delegadas todavía deben revisarse; no contarlas automáticamente en los 16 anteriores.

Se recibió la entrega adicional de seis tests de hardening con resultados anteriores a los fixes. La reejecución parental detectó tres defectos de fixture/assertion: un supuesto reporte positivo usaba `read committed`/autoridad incompleta, el comando positivo duplicaba el prefijo gcloud, y la comparación de intención/delete usaba índices de listas diferentes. Se corrigieron únicamente tests para usar el contrato real y una única secuencia de eventos; no se relajó código de producción. Suite Python completa posterior: **97 tests PASS** (`$TMPDIR/espacigo-recovery-python-final.log`), con ResourceWarning HTTPError de fixtures. `git diff --check` PASS. Los **7 archivos de producción congelados** conservan todos sus hashes; las revisiones independientes pendientes siguen aplicando al mismo candidato.

### Gate IAM observado: NO listo para ejecución remota

La metadata IAM de solo lectura muestra que el provisioner tiene `espacigoReleaseJobProvisioner` y logWriter; su custom role permite create/get de Jobs, builds.get y lectura de metadata de bucket, pero no executions.get/list. En el bucket, su objectViewer está condicionado únicamente a `source/` y `releases/`; el lock `cloudrun-main.lock` y las nuevas solicitudes `recovery-inspections/` quedan fuera. El Job aplicador histórico no tiene bindings directos para esa identidad. Evidencia: `evidence_recovery_readonly_iam_20261107.json`.

Eso demuestra que **los bindings inspeccionados no bastan**, no una denegación efectiva global: existe un padre de organización y no se han descartado todos los permisos heredados/grupos. La consulta READ ONLY a Policy Troubleshooter no pudo evaluar acceso porque la API está deshabilitada (`SERVICE_DISABLED`); no se habilitó. No se modifica IAM ni se relaja el gate para seguir.

Antes de ejecutar el build/Job: demostrar autoridad existente suficiente o recibir autorización explícita para un delta IAM de lectura temporal, acotado por recurso y con revocación verificable. No conceder permiso de ejecutar/actualizar Jobs al provisioner, acceso a payload SQL, DDL o roles de runtime. La autorización de publicación de la inspección no se amplía por inferencia a IAM/APIs. Continúan sin commit/push, build nuevo, Job nuevo, conexión SQL cloud, desbloqueo o despliegue.

### Revisión final: publicable solo en rama, ejecución aún bloqueada

Las dos revisiones independientes `deleg_0a1dba5b` aprobaron Python/configs y Go/CLI sin hallazgos concretos; comprobaron los siete hashes congelados. Recibo versionado: `evidence_recovery_readonly_code_review_20261107.json`. La revisión parental revalidó esos hashes y `go test -count=1 ./...` PASS, además de los 97 Python/ensayo real/vet anteriores. Aprobación de fuente no cierra IAM ni demuestra lectura Cloud SQL actual.

El nombre local inicial `codex/database/fix/recovery-readonly-inspection` no cumple el identificador numérico exigido por el hook versionado. Para publicar el mismo trabajo autorizado sin omitir/editar controles, se usa `codex/database/fix/recovery-01-readonly-inspection`, mediante renombre de la rama propia, sin reescribir commits, crear PR o tocar main. El trigger GCP vigente `548b8eff-e489-4307-a77f-455b56aa4e57` se leyó por metadata y selecciona únicamente `^main$`; GitHub Database migration quality sí selecciona `codex/**` y ejecuta ensayos locales del runner, no un release Cloud SQL. La publicación de esta rama no habilita ejecución ni adaptación a main.

Código revisado/congelado en commit `09d3430f424241d17c2a264b3f305859377117c0`. Se ensayó además una extracción **solo versionada** del árbol preparado `192e10f34ab476499b389fbfbb76095bc292ddda`: Go de CLI/migrator/CloudSQL y los **97 tests Python PASS** sin archivos privados/ignorados. `bash scripts/test-runtime-migrator-image.sh` compiló localmente el commit anterior y verificó las **84 migraciones/nombres/checksums**, SHA embebido y rechazos de target/modos sin acceso SQL. Tag propio de imagen eliminado y ausencia comprobada. El registro posterior de estos resultados cambia solo documentación; no altera los siete archivos de producción revisados. No se ha construido imagen cloud ni abierto una nueva ejecución privada.

### Publicación verificada y autorización IAM posterior

Se publicaron código y evidencia únicamente en `codex/database/fix/recovery-01-readonly-inspection`, HEAD `cd5efb778837ec3c22023eadc6b3858030dd735d`; remoto leído y coincidente, autor/committer `HernanEspinozaDev`, hooks activos y worktree limpio en ese punto. `main` remoto siguió en `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`. CI Database migration quality **42418129420 SUCCESS** para ese HEAD: guards/inventario, verificación de evidencia, ensayo PostgreSQL obligatorio y compilación/inspección local de imagen. Esto no es despliegue ni aceptación de producto.

Ante la pregunta concreta de ampliar únicamente las lecturas necesarias del provisioner, el usuario seleccionó literalmente **«Autorizar IAM de lectura temporal y acotado, con revocación verificada»**. La pregunta autorizada limita el delta al lock retenido, la solicitud **de esta inspección** y los metadatos del Job fallido; exige preparar/revisar antes de aplicar y revocar con readback al terminar. **No incluye** SQL/ACL de DB, payload de contraseñas, ejecutar el Job aplicador, unlock ni despliegue. Esta decisión posterior sustituye solo la ausencia de autorización IAM indicada anteriormente; no convierte permisos no examinados en autoridad demostrada.

Tras esa autorización se reejecutó el inspector de metadata `inspect_failed_release` como operador, sin SQL: lock generación **1794036269394395**, owner build **2dc002a9-5831-4717-8064-6432b98d3e01**, source **bb10309129adad9d7204b9b54bb2259186a34242**, único aplicador histórico **...-v62zb** terminal fallido. No se creó nueva ejecución. El delta IAM aún está en preparación/revisión; no se aplicó ningún grant ni se habilitó API. No usar un wildcard de solicitudes futuras para eludir que el build ID padre solo se conoce tras crear el build.

### Primer parent manual y bloqueo de bootstrap demostrado

Se aprobaron y publicaron fuente/IAM helpers en **44a4a205c572df0c96a1027a6a3bb62e0497c018**, solo rama de tarea; CI **42422269416 SUCCESS**. El archive Git del commit coincide byte a byte con sus **2898 archivos** y se verificó también tras lectura GCS por generación: SHA-256 **7cbdc4733ed8cfc970e5f937c450fe85e24aee2c761370680b21b8029491a95a**. Intención/readback del submit se conservan privados en `.local/recovery-operator/44a4a205c572df0c96a1027a6a3bb62e0497c018/`.

Parent manual **277fcc60-2c97-476b-b4e5-3377eb94b8ff**: **FAILURE**, salida **127** antes de iniciar Python, `bash: line 3: python3: command not found`. El SDK fijado dispone de Python bundled, no del ejecutable global asumido. Readbacks: Job propio de ese parent ausente, prefijo de roles temporales vacío. **No hubo grant IAM, image child, ejecución Job, conexión SQL, unlock ni rollout**. Recibo público: `evidence_recovery_parent_277fcc60_20261107.json`. No se relanza ese build ni se reutiliza su identidad.

Se reprodujo el fallo en RED con **la misma imagen SDK** y las cabeceras YAML reales en un contenedor local sin red/cuentas/credenciales. Se corrigieron ambas configuraciones dedicadas y el contrato `parent_step` siguiendo el patrón existente de `cloudbuild.yaml`: obtener `basic.python_location`, comprobar ruta ejecutable y usar CA certifi de la imagen. La conversión `$$`→`$` se comprobó contra metadata real del pipeline histórico. `scripts/test-recovery-sdk-image.py` pasa ambos entrypoints/imports reales con `--help`, sin ejecutar el flujo cloud, y confirma retirada de los contenedores propios. **127 tests Python PASS** tras la corrección. No se instala Python ni se cambia la imagen/procedimiento normal.

Corrección de bootstrap congelada para revisión en `deleg_f49e63b9` antes de publicar/preparar un parent nuevo del SHA corregido. La lógica IAM ya revisada permanece byte a byte intacta. El ledger actual Cloud SQL sigue **sin lectura nueva**; no convertir el fallo de bootstrap en evidencia de estado SQL.

Entorno reproducible del smoke: `TMPDIR=/home/nandev/.hermes/cache/scratch PYTHONDONTWRITEBYTECODE=1 /usr/bin/python3 scripts/test-recovery-sdk-image.py`; `/usr/bin/python3` tiene PyYAML **6.0.3**, comprobado por import real. `TMPDIR` es un prerrequisito explícito, no se usa fallback a `/tmp`. La primera revisión confirmó hashes/seguridad/contrato SDK pero su intérprete alternativo carecía de `yaml`, de modo que no pudo reproducir el smoke. La comprobación independiente del comando/entorno exactos continúa en `deleg_30dc34ef`, sin cambiar los archivos congelados ni instalar paquetes.

Resultado posterior `deleg_30dc34ef`: **PASS**, cuatro hashes iguales, ambas pruebas SDK reales offline ejecutadas con éxito y ausencia de contenedores propios comprobada; sin hallazgos de seguridad/lógica. Gate de fuente/bootstrap cerrado, recibo `evidence_recovery_sdk_review_20261107.json`. Queda la publicación del SHA corregido y un parent manual nuevo, seguido por request exacto, IAM temporal/readback/efectividad y retirada comprobada. El parent fallido anterior no se reejecuta.
