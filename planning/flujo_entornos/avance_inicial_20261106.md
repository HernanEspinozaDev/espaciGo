# Ejecución inicial FLUJO-01–03 — 2026-11-06

Encargo: «comienza las fases». Rama `codex/infra/feature/flujo-01-migraciones-sin-bastion`, creada desde main `771134d165046b290b1f5ad3a1074a6357c2327f` e incorporando por fast-forward la planificación `b189beda721392499eb19e692f6c0a000ea90b57`. Sin cambios en main, trigger de despliegue, datos SQL, IAM o recursos cloud al escribir esta primera entrada.

## FLUJO-01: inventario real de solo lectura

Evidencia proyectada y sin payloads: [inventario cloud](evidence/flujo01_inventory_20261106.json). Las respuestas originales permanecen en el directorio privado excluido del worktree; no contienen solicitudes de valores de secretos.

- Identidades verificadas: Git/gh `HernanEspinozaDev`; cuenta GCP coincide con el operador del runbook.
- SQL: PostgreSQL 18, ENTERPRISE, `db-g1-small`, ZONAL, SSD de 10 GB; RUNNABLE, solo IP privada, misma instancia/base histórica. El tamaño real difiere del candidato Terraform antiguo; no se utiliza aquel para cotizar.
- Backups automáticos habilitados con retención **7 copias** (COUNT, no prueba de 7 días exactos). Último backup `1793940180000`, AUTOMATED/SUCCESSFUL, fin `2026-11-06T06:07:24.449Z`, ubicación `us`. PITR deshabilitado; `transactionLogRetentionDays=7` no lo activa.
- La subred API `10.81.0.0/26` tiene Private Google Access. API usa Direct VPC egress private-ranges-only; por tanto el Job puede seguir esa arquitectura tras verificar conectividad/identidad, sin conector VPC dedicado.
- No hay Cloud Run Jobs ni migration jobs DMS en `southamerica-west1` al capturar. Esto no demuestra ausencia de sesiones locales/otras dependencias del bastión.
- Bastión e2-micro TERMINATED, un disco pd-standard 10 GB con autoDelete; sin snapshots del disco ni IP reservada asociada en las listas consultadas. No se inició ni se inspeccionó su sistema operativo.
- Trigger main presente, sin aprobación configurada; deployer custom no tiene run.jobs.* ni acceso a payloads de secretos por ese rol. Existe una cuenta histórica `espacigo-test-migrator`, que no se reutiliza por inferencia. Nueva identidad propuesta separada: `espacigo-db-migrator`.
- Billing habilitado; no se consultó exportación de facturas ni se atribuye costo mensual real. Ledger/ACL/autoridad DDL siguen pendientes de lectura por la nueva ruta; V83 remoto es todavía evidencia anterior.

No se cierra recuperación por observar SUCCESSFUL: falta restauración ensayada. La configuración SQL permite conexiones sin TLS, pero el nuevo transporte utilizará el conector TLS; no se cambió esa política ni se presenta el ajuste como requisito de conectividad.

## FLUJO-02: primera cobertura implementada

- Harness versionado `scripts/test-migration-runner-postgres.sh`: cluster PostgreSQL 18.6/PostGIS fijado por digest, red none, sin puertos host ni volúmenes persistentes; socket privado, limpieza solo por ID del contenedor propio.
- Ensayo real de instalación V1–V83 y actualización V75→V83. V81 conserva su guard y usa login migrador/SET ROLE; la preparación de roles y la ventana V82/V83 son fixtures sintéticos de administrador, no repairs cloud ni evidencia de autoridad remota.
- Se comprueban plan READ ONLY, reejecución sin cambios, checksums, huecos, fallo/rollback por archivo, reanudación y exclusión entre conexiones/procesos. El checker exige 14 resultados concretos y rechaza pruebas omitidas/ausentes.
- Workflow `database-quality.yml` sin credenciales cloud: configura Go, ejecuta el harness y comprueba la imagen; cubre main/ramas codex para paths relevantes. Está preparado en fuente, no se afirma ejecución remota hasta consultar su run.
- Manifest offline de release liga SHA completo, repositorios/digests de API/web/migrador, destino fijo e inventario SQL. El verificador compara prefijo aplicado y sufijo pendiente exactos y exige cero pendientes en postcheck. No verifica por sí solo provenance remoto, antigüedad de una captura ni compatibilidad funcional de la API anterior/nueva: esos gates siguen pendientes de integrar.

**Estado FLUJO-02: parcial.** Las pruebas de migrador/esquema y contratos de inventario están ejecutadas; falta completar contrato funcional API–esquema y conexión de sus resultados al gate de despliegue del mismo SHA.

## FLUJO-03: imagen actual y propuesta de Job

Se corrigieron dos defectos de empaquetado:

1. El Dockerfile actual copiaba solo V1–V69. Ahora incluye todo el directorio y valida inventario durante build.
2. `go mod download` fallaba porque `go.mod` reemplaza pdfcpu por un módulo local no copiado. Se incluyen únicamente sus archivos de módulo para resolver dependencias del migrador; la compilación real pasó.

La CLI incorpora `-inventory-only`, que corre antes de cargar configuración DB y no conecta a nada. Rechaza combinarlo con plan/aplicación/modos administrativos. Se ejecutó la imagen scratch real con network none/read-only y se contrastaron los 83 nombres/checksums contra fuente; también se probó rechazo de modo conflictivo.

`scripts/gcp/migration_job_plan.py` genera solo una propuesta de Job READ ONLY, destino fijo, image digest obligatorio, versión numérica del secreto, 1 tarea/paralelismo1/reintentos0 y cuenta separada. No ejecuta gcloud ni admite un modo de aplicar. `cloudbuild-migrator-image.yaml` publica únicamente la imagen migradora con provenance exigida; no despliega aplicación ni ejecuta SQL.

Para la prueba cloud siguiente: nueva cuenta `espacigo-db-migrator`, Cloud SQL Client **condicionado a la instancia exacta** según IAM Conditions de Cloud SQL, Secret Accessor solo sobre el secreto migrador existente, y Job `espacigo-db-plan`. No otorgar al deployer autoridad para ejecutar con overrides ni contraseñas SQL. Provisionar y leer vuelta los grants/configuración antes de invocar el Job; la propuesta no acredita que existan.

**Estado FLUJO-03: empaquetado local y propuesta preparados; provisionamiento/verificación cloud pendientes.**

## Validación ejecutada

- `go test ./cmd/dbmigrate ./internal/migrator ./internal/database/cloudsql`: PASS (unitarias; integraciones opt-in se registran aparte).
- `go vet` sobre esos tres paquetes: PASS.
- Harness PostgreSQL: PASS, 14 resultados obligatorios sin skips; dos recorridos hasta V83 y conservación del historial previo.
- Imagen migradora local: PASS, inventario fuente/imagen de 83 SQL idéntico, modo conflictivo rechazado sin red/DB. Primer build descubrió el módulo local omitido y se corrigió; segundo build real pasó.
- Regresión Python de `scripts/gcp/test_*.py`: PASS, 42 pruebas.
- Validador de la transición Cloud Build existente: PASS. No se cambió `cloudbuild.yaml` ni su comportamiento activo.
- YAML de los dos nuevos archivos parseado; `git diff --check` PASS. Contenedores y tags propios del ensayo retirados por los harness; caché compartida no purgada.

## Gates siguientes

1. Publicar el código revisado en la rama y construir la imagen del SHA limpio con provenance en el registro actual; verificar digest/inventario/source antes de configurar Job.
2. Probar plan READ ONLY cloud sin bastión y registrar ledger/autoridad efectivos. Mantener pendiente aplicación automatizada hasta recuperar/validar backup y completar contrato de release.
3. Preparar recuperación aislada y la transición de trigger sin dos rutas concurrentes; todavía no deshabilitar el trigger ni desplegar main.
4. Mantener FLUJO-04–07 abiertos; bastión retenido detenido hasta sustituir diagnóstico/recuperación y comprobar dependencias para su retirada exacta.

## Continuación ejecutada: imagen cloud y primer Job READ ONLY

Se avanzó dentro del encargo de iniciar las fases con recursos acotados del piloto; la entrada inicial anterior conserva el estado que tenía al publicarse. Evidencia: [preflight privado por Job](evidence/flujo01_private_job_preflight_20261106.json).

- Código de imagen `3d4b7ec6bed4df2dbfb45fca78dc7a0d476d9592`; build cloud `d855a82d-9910-4335-9e6a-be9d0a0803d2` SUCCESS. Se verificó provenance SLSA del build exacto y digest `sha256:4091c049987a28f989523f9c4993dbcc43e42468d39fddab775c06a69555f1aa` antes de configurar el Job. Solo se publicó el migrador; no API/web.
- Se creó `espacigo-db-migrator@espacigo-testprod.iam.gserviceaccount.com`, sin claves de servicio, con Cloud SQL Client condicionado a `projects/espacigo-testprod/instances/espacigo-testprod-db` y al tipo Instance; Secret Accessor se asignó solo al secreto migrador existente. El acceso IAM al secreto abarca ese recurso, no se afirma que esté limitado por IAM a una única versión; el Job monta específicamente versión1. No se amplió el deployer ni las identidades runtime.
- Se creó `espacigo-db-plan`, 1 tarea/paralelismo1, reintentos0, timeout600s, 1CPU/512Mi, Direct VPC por la subred API, command/args fijados a `-plan-only`. Readback confirmó imagen por digest, identidad y modo antes de ejecutar. Ningún override de ejecución.
- Ejecución `espacigo-db-plan-hxnv9` completada correctamente; fin UTC `2026-11-07T00:03:38.988108Z` (2026-11-06 en America/Santiago). Reportó PostgreSQL18.6, transacción READ ONLY y ledger V1–V83/cero pendientes. Se compararon las 83 entradas completas con la fuente, sin aceptar solo el máximo de versión.
- La primera ejecución CI `42315321541` falló porque el runner no tenía rg: fue un defecto del harness, no una migración. Se corrigió usando grep estándar y diagnósticos de startup, commit `32d43f29336168ce54aed6e55747f2554cc9fcc6`; se repitieron ambos harness locales. CI corregido [42315712474](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/42315712474) terminó SUCCESS e incluye las pruebas PostgreSQL y la imagen. El código del binario y los SQL no cambiaron entre esos dos commits.

**No hubo aplicación de SQL**, cambio de política backup/PITR, restore, integración a main, despliegue de aplicación, cambio de trigger ni encendido/eliminación del bastión. La lectura prueba conectividad/autenticación/SET ROLE y guards de catálogo del plan; no demuestra autoridad DDL para cualquier futura migración. El manifest offline aún no es un gate activo de Cloud Build.

Recursos retenidos deliberadamente para continuar FLUJO-04/05: la cuenta/grants nuevos, Job de plan sin ejecuciones activas y la imagen en el registro actual. Responsable: tarea FLUJO/operador del proyecto. Propiedad de este piloto: configuración operacional registrada y generador versionado; aún no hay importación a state Terraform ni dueño automatizado del ciclo de aplicación. Resolver esa propiedad en la activación antes de administrar Job/IAM por dos herramientas.

Estado tras el piloto: inventario FLUJO-01 y lectura SQL actual documentados; FLUJO-02 mantiene pendiente compatibilidad funcional API–esquema y wiring de calidad/despliegue; FLUJO-03 tiene imagen y Job de plan operativos, falta Job de aplicación/circuito; FLUJO-04 inició su preflight real pero sigue pendiente restore; FLUJO-05–07 permanecen pendientes. El bastión **no se elimina** antes de ensayar recuperación y resolver dependencias. No hay aceptación funcional online por esta evidencia.

## Continuación FLUJO-02/04/05: recuperación y gate de calidad

La siguiente ejecución sustituye los pendientes de restore y cobertura indicados en las entradas anteriores, conservadas como historial. Evidencia: [ensayo de recuperación](evidence/flujo04_restore_20261106.json).

- Se restauró el backup gestionado `1793940180000` en una instancia **nueva y temporal**, `espacigo-flujo04-restore-20261106`, privada y etiquetada. Restore `99be0db4-5de7-4fd2-b55e-40070000004b` terminó el `2026-11-07T00:28:35.143Z`. No se restauró sobre el origen.
- Job temporal `espacigo-db-restore-plan-dgtq4`: READ ONLY PostgreSQL18.6, V1–V83/nombres/checksums idénticos a fuente, cero pendientes. Solo comprobaciones de esquema/ledger y guards del migrador: no se exportaron datos ni credenciales y no se ensayó conmutación funcional de la aplicación.
- Se eliminó el Job temporal, se retiró su binding Client exacto con expiración y se eliminó la instancia; operación de borrado DONE a `2026-11-07T00:36:30.752Z`, GET de instancia/Job devuelve ausencia. El permiso original del piloto permanece. Postcheck del origen `espacigo-db-plan-qc4qn`: mismas 83 entradas, cero pendientes. Bastión permanece TERMINATED.
- El generador `restore_rehearsal_plan.py` conserva comandos revisables y guard de nombre/proyecto/etiquetas/red antes de restaurar o borrar; no ejecuta comandos. El SDK estable rechazó labels antes de crear recursos; se usó beta para crear con etiquetas desde el inicio. No reutilizar estos identificadores como un procedimiento recurrente sin definir una nueva sesión y verificar ausencia.
- Referencia de catálogo consultada para Santiago: PostgreSQL Zonal Small USD0.049/h de cómputo; no incluye almacenamiento/Jobs/logs ni acredita factura real. La instancia se mantuvo solo durante el ensayo. Backups automáticos del origen siguen habilitados, retención7copias, PITR deshabilitado. Restaurar un backup diario no acredita recuperación a un segundo determinado ni RTO de negocio.

### Cambios implementados en fuente

- Contrato API–esquema sobre PostgreSQL desechable: repositorios reales de identidad, catálogo y verificación documental bajo el rol runtime; prueba positiva y negativa del permiso de bloqueo V83, sin cuentas ni datos reales. Corre tanto desde cero como desde V75. El harness exige ahora **16 resultados**, sin skips; no acredita todos los endpoints ni compatibilidad de cada binario histórico.
- Nuevo primer paso de Cloud Build verifica workflows requeridos del **mismo SHA de main** y rechaza fallos/skips, evidencia antigua/ajena y candidatos que dejaron de ser HEAD. Espera acotada; error de API falla cerrado. Solo consulta la API pública de Actions, sin credencial. Todas las etapas posteriores conservan secuencia y fallo obligatorio.
- Web exige calidad de aplicación/frontend; full añade database-quality. Permanecen los controles existentes de backend sin cambios y evidencia de esquema. Este gate está en la rama; no está activo en el trigger hasta integrar el corte. **No aplica migraciones** ni sustituye aún el circuito completo de release.
- Se detectó que frontend-quality del main base estaba rojo por formato de dos archivos: se corrigió exclusivamente whitespace. No hay rediseño ni comprobación online. Formato, lint, TypeScript y 449 pruebas frontend locales PASS; sin skips.

### Verificación y límites de cierre

PostgreSQL: 16 resultados PASS; unitarias/vet Go pertinentes PASS. Regresión Python de GCP: 49 pruebas PASS, más 3 pruebas del orden de calidad Cloud Build. Validador de transición de 9 pasos PASS; gate de esquema en SDK real fijado por digest PASS con evidencia válida/rechazo de drift. `git diff --check` PASS. Fixtures de los tests de Cloud Build son sintéticos; sus mensajes de servicios/triggers no son despliegues nuevos.

FLUJO-04 tiene ensayo real de backup→instancia→esquema/ledger y limpieza completados; quedan decisiones de retención/RPO, recuperación funcional y PITR según riesgo. FLUJO-02 mejora su contrato SQL de API pero no cierra toda compatibilidad funcional. FLUJO-05 sigue parcial: faltan Job de aplicación, enlace de manifest/provenance/reportes y exclusión de releases antes de activar la ruta automática. No se integró main, no se cambió trigger/aplicación, no se aplicó SQL al origen, no se retiró bastión. La comprobación online sigue reservada al usuario.

Código revisado/publicado: `3d494762b8957b086dd19fcc214d0adc174f3516`. [Database migration quality 42319793198](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/42319793198) terminó SUCCESS para ese SHA, incluidos los 16 resultados obligatorios y build/inspección de imagen. Application/frontend no se ejecutan remotamente para esta rama por sus filtros main; su validación citada arriba es local. Este registro documental posterior no altera el código probado ni activa main.

## Continuación: modo release del aplicador y verificación de ejecución

Se implementó el [contrato del Job aplicador](contrato_job_aplicacion_20261106.md): SHA incorporado al binario, inventario exacto, prefijo revisado, destino cloud allowlist y preflight/aplicación/postcheck en una conexión bajo el mismo advisory lock. Se conserva el runner transaccional y todo SQL histórico byte a byte. El modo no acepta opciones administrativas y no emite éxito tras fallos parciales.

Se añadió generador/verificador offline del Job único por build: espera real sin overrides y comprobación de ejecución exacta, imagen, cuenta, red, secreto/version, tarea completada y reporte consistente. No ejecuta cloud ni sustituye provenance, backup, exclusión global o el catálogo IMG-01; su integración está pendiente.

Pruebas locales: PostgreSQL desechable **21 resultados obligatorios PASS**, incluidos no-op V83 con identidad restringida en fresh/upgrade, competencia de releases y fallo parcial con rechazo de reintento obsoleto. Unitarias/vet Go PASS; Python GCP **54 pruebas PASS**; imagen scratch real PASS (inventario/byte a byte, SHA incorporado y rechazo de destino/modos). Validador Cloud Build PASS, `git diff --check` PASS. Recursos propios del ensayo retirados; no se tocaron bases ni volúmenes persistentes.

Este corte es código/ensayos locales: no se creó un Job aplicador cloud, no se ejecutó SQL remoto, no se construyó una imagen cloud nueva ni se cambió IAM, trigger, main o aplicación. FLUJO-03 completa contrato local del aplicador; FLUJO-05 sigue abierto hasta conectar y verificar el circuito completo descrito en el documento. Bastión sigue retenido detenido según el último inventario; no se acredita una nueva lectura cloud por esta entrada.

Código revisado/publicado de este corte: `0b7853a3c73a6d330022cbc2558d1b259a9708f6`. [Database migration quality 42321456345](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/42321456345) terminó SUCCESS para ese SHA, incluidas las 21 comprobaciones PostgreSQL obligatorias y la imagen con SHA incorporado. Los otros workflows conservan sus filtros de main; no se afirma ejecución remota de aplicación/frontend por este push.

## Continuación: catálogo IMG-01 y gate de backup

El migrador incorpora `-schema-evidence-only` para recoger catálogo/ledger en transacción READ ONLY mediante el conector privado y la identidad migradora, sin bastión. La imagen incluye la consulta IMG-01 original byte a byte; se retira únicamente su framing psql al ejecutarla con pgx y se conserva el hash del archivo completo en el reporte. El modo release emite ahora `espacigo.migration-execution.v2` con el catálogo capturado bajo su lock de sesión después del postcheck. Un fallo de catálogo no produce éxito aunque SQL anterior ya haya confirmado; se conserva el rechazo del reintento obsoleto.

El verificador de ejecución reutiliza todas las reglas IMG-01 existentes, sin quitar controles de roles/ACL/objetos/ledger; exige además identidad migradora, captura dentro de la ejecución e inventario exacto del manifest. La validación del JSON se extrajo como función reutilizable manteniendo la CLI anterior.

`backup_readiness.py` valida metadata nativa reciente de instancia/backup. Política inicial implementada para el desarrollo actual: captura de metadata de hasta5min, backups automáticos habilitados con al menos7copias COUNT y backup satisfactorio iniciado hace como máximo24h. PITR debe estar observado explícitamente, pero no se exige activarlo en esta política inicial. Si hay SQL pendiente, se requiere backup ON_DEMAND iniciado después de tomar el lock global de release. Esta política no promete un RPO de producción y se revisará antes de los primeros usuarios. `prepare_application` y la CLI de propuesta del Job exigen ese gate; el renderer de configuración básico no ejecuta nada y mantiene sus prerrequisitos externos.

Lectura cloud de este corte: instancia/backup nativos reconsultados, backup1793940180000 pasa el gate sin SQL pendiente. No se creó backup ni se cambió su retención/PITR. Ensayo del nuevo catálogo cloud pendiente al escribir esta entrada; no se declara la ruta activa por preparar código.

Pruebas locales PASS: 25 resultados PostgreSQL obligatorios, incluido bloqueo de SELECT que intenta escribir y fallo de catálogo posterior a DDL; 60 pruebas Python GCP y 7 del gate IMG-01 existente; unitarias/vet Go; imagen scratch real con consulta empaquetada idéntica y rechazo de modos conflictivos; validador de transición y diff-check. Las pruebas de catálogo del fixture adaptan solo el predicado de nombre de base al nombre aleatorio local y no acreditan lectura del destino cloud. Sigue pendiente la orquestación completa y transición del trigger; este corte no integra main ni activa SQL automático.

### Ensayo cloud del catálogo completado

Código `a5cd62549a47baca91e5d7cde8cabc05306876e3`; [CI 42323120571](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/42323120571) SUCCESS. Build migrador `8bffdba9-62e2-432a-8317-d8b0fc724279` SUCCESS; digest `sha256:79826c2f91014ddc6af0c532614f5484a72c1313d57cb91576c75a3de347d812`. Fuente enviada mediante `git archive` del commit limpio, sin archivos locales ignorados; SHA256 del archivo coincide con el hash nativo de sourceProvenance y se comprobó la provenance SLSA v1 del build/digest exactos.

Job temporal `espacigo-schema-plan-a5cd62549a47` con modo exclusivamente `-schema-evidence-only`. Se comprobó configuración efectiva antes de lanzarlo sin overrides y se conservó ID de ejecución `espacigo-schema-plan-a5cd62549a47-xk6vf`; finalizó con una tarea satisfactoria el `2026-11-07T01:21:12.788869Z`. El catálogo cloud pasó **todas** las reglas IMG-01 existentes: PostgreSQL18, ledger83/nombres/checksums exactos, roles/ACL/objetos y hash de la consulta fuente original. Login migrador/SET ROLE schema_owner, transacción READ ONLY y captura dentro de la ejecución comprobados. No hubo lectura de negocio ni recuperación local de payloads de secretos.

El Job temporal se eliminó y su ausencia se comprobó a las `2026-11-07T01:22:06.737813+00:00`; archivo local de source retirado. Se conserva deliberadamente la imagen publicada y los registros del build para la continuación del circuito. No se añadieron grants IAM, se conserva el Job de plan anterior y no se aplicó SQL. Trigger reconsultado coincide con la captura anterior; bastión revalidado TERMINATED. No se desplegó aplicación ni se hizo QA online.

Evidencia: [catálogo por Job y limpieza](evidence/flujo05_catalogo_job_20261106.json), [observación del gate de backup](evidence/flujo05_backup_gate_20261106.json). La observación de backup es histórica y no sirve como recibo reutilizable en un release posterior. La regresión del gate de esquema en su SDK real fijado por digest también PASS tras el refactor.

Se cierra la brecha técnica de **captura/validación de catálogo desde el Job**, que antes exigía una captura manual separada. FLUJO-05 permanece parcial: falta enlazar en un único orquestador activo lock global por generación, consultas/backup bajo demanda, configuración/ejecución del Job, persistencia de evidencia y despliegue por digests. También quedan definir limpieza de backups ON_DEMAND, reconciliar configuración real/declarada y completar la transición del único trigger antes de integrar main. La nueva política de backup está preparada y probada, no modifica la instancia ni activa el pipeline.

## Cierre de implementación y ventana de activación

Encargo humano: «continua hace lo que falta para terminar». Se completa el orquestador único `release_main.py` con clasificación por inventario, lock global con espera del build activo/precondición por generación, imágenes y provenance, Jobs efímeros con configurador separado, backup/postcheck/catálogo, evidencia inmutable y promoción técnica. Se mantiene íntegra la configuración efectiva de API/web (incluido maxScale10 de API, distinto del YAML anterior1); IAM de aplicaciones/red/secretos no se reescriben.

Identidad `espacigo-release-job-builder` y roles suplementarios configurados mediante script/JSON versionados, independientes del role Terraform previo: padre puede lanzar/observar, configurador solo crear; no overrides/update ni payload SQL. Se conserva autoridad efectiva delegada DDL como frontera explícita de Git/main. Sin grants SQL nuevos. El proveedor confirmó que `cloudsql.operations.get` no existe; se utiliza la autoridad de instancia requerida por SQL Admin para observar operaciones, sin añadir ese permiso ficticio.

Los SQL futuros requieren política exacta de expansión y pruebas fresh/upgrade; el primer release ejecutará no-op V83, sin inventar V84. La retención ON_DEMAND se limita a recibos de release exitoso y descripción exacta del build; conserva7 más recientes y≥7d, protege backup actual y deja ajenos/históricos/fallos fuera de limpieza.

Pruebas de orquestación: bloqueo ante backup/Job fallido, ownership del lock y recursos, readiness y labels/digest, ausencia de despliegue docs, clasificación web, política de SQL desconocido, preservación de configuración y rollback de tráfico ante promoción incierta. La cobertura ejecutable del YAML anterior se conserva como fixture histórico; la ruta activa se prueba en el ejecutor Python. Full Go local PASS usando Poppler administrado por root en `/usr/bin`; el PATH ambiental resolvía antes un override no confiable y fallaron dos pruebas de confianza. No se debilitó ese guard. Vet completo, regresión Python, gate SDK y validación Cloud Build PASS.

Activación pendiente al escribir esta entrada: pausar trigger con ausencia de builds activos, integrar normalmente a main, esperar sus tres workflows, reactivar único trigger y ejecutar candidato controlado. Solo tras resultado técnico y diagnóstico/recuperación comprobados se retirará el bastión exacto; no se da por eliminado ni se inventa QA funcional.
