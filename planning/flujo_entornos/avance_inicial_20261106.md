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
