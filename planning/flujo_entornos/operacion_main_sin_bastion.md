# Operación inicial: main → Cloud SQL/Cloud Run

El código de este corte sustituye el orquestador anterior. La activación observada y su recibo se registran en `avance_inicial_20261106.md`; no dar por desplegado un candidato por publicar su rama. Sin develop ni staging en esta etapa.

## Publicar cambios

1. Crear/revisar/probar el corte en rama temporal; conservar SQL aplicado byte a byte.
2. Para SQL nuevo, añadir migración y su entrada exacta en `db/release-policy.json`: `{"checksum":"SHA256_DEL_SQL","strategy":"expand"}`. Actualizar la receta fresh/upgrade de CI y verificar autoridad DDL del migrador para los objetos afectados. La política no concede permisos SQL y rechaza deltas desconocidos/destructivos.
3. Integrar normalmente a main, sin PR ni force-push. Un único trigger espera CI del mismo SHA; errores de calidad bloquean despliegue.
4. El clasificador compara inventario de código con el último release exitoso: documentación no despliega ni toca SQL; frontend usa ruta web que prueba backend intacto; cualquier otro cambio exige ruta completa.
5. La ruta completa toma lock por generación, verifica digests/provenance de API/web/migrador, ejecuta plan privado, verifica backup y aplica solo pendientes. El catálogo IMG-01 debe pasar sus reglas completas. Solo después crea todas las revisiones sin tráfico y promueve API/web. Se conserva la configuración efectiva de los servicios, incluidos secretos, red, IAM, escalado y probes.

El ledger no es una copia de la base local: los datos permanecen en Cloud SQL. El primer release sin pendientes es válido y no requiere inventar V84. Cambiar un struct Go o una tabla manualmente en local no crea una migración.

## Identidades y evidencia

- `espacigo-cloud-build-deployer`: construye imágenes, lanza/observa Jobs y despliega servicios. No recibe payloads SQL ni permiso para actualizar Jobs/usar overrides.
- `espacigo-release-job-builder`: child build separado que valida request inmutable/parent activo y crea Jobs. Puede actuar solo como migrador para este fin; no ejecuta/actualiza Jobs ni lee secretos directamente.
- `espacigo-db-migrator`: identidad runtime con Cloud SQL Client condicionado a instancia exacta y acceso al secreto migrador existente; SQL login/SET ROLE existentes. No se otorgó superusuario ni permisos nuevos a runtime API.

La delegación del configurador es autoridad efectiva de migración: Git/main y sus cambios de pipeline son la frontera de confianza. No se presenta como aislamiento frente a código malicioso autorizado en main. Los roles suplementarios y bindings de este corte son propiedad de `bootstrap_release_identity.py`/`infra/releases`; no se cambió el custom role Terraform previo ni se administró ese recurso por dos herramientas.

Cada build conserva bajo `gs://espacigo-testprod-cloud-build-447373903153/releases/<UUID>/` manifest, baseline, backup, solicitudes/intenciones/ejecuciones exactas, logs de catálogo, postcheck y resultado. Los objetos se crean con precondición y se releen por generación/hash. El watermark exitoso liga código, imágenes, esquema y revisiones. La contraseña se monta solo en el Job, nunca se registra en estos objetos.

## Fallos y recuperación sin túnel

No volver a ejecutar un Job por faltar logs o por cancelarse el build padre. Leer ID de ejecución, su estado, configuración efectiva y ledger/catálogo desde el ejecutor privado. Cada release conserva lock ante resultado incierto; releases concurrentes esperan al que sigue activo y nunca eliminan el lock de un build fallido.

Para fallos demostrados **antes** de la aplicación:

```bash
python3 scripts/gcp/recover_release_lock.py --account CUENTA_OPERADORA
python3 scripts/gcp/recover_release_lock.py --account CUENTA_OPERADORA --unlock-preapply
```

El segundo comando solo libera la generación observada si el padre canónico terminó fallido, no existe intención de aplicación y no hay ejecución aplicadora. Ante escritura posible, se rechaza: inspeccionar resultados reales antes de un nuevo candidato. No alterar ledger ni usar bootstrap admin genérico para saltar un error. Aplicación fallida tras migración requiere revisión anterior compatible o corrección hacia adelante; SQL no se revierte automáticamente.

Un fallo durante la promoción revierte el tráfico ya promovido a las revisiones previas registradas, bajo la política expand; el lock y la evidencia permanecen para diagnóstico. Readiness es control técnico de Cloud Run; QA/aceptación online corresponden al usuario.

## Backups y costos

Se conserva la instancia de desarrollo actual. Sin SQL pendiente se admite backup exitoso de hasta24h y metadata de hasta5min, con automáticos habilitados y ≥7copias. SQL pendiente requiere un ON_DEMAND exitoso iniciado después del lock. PITR sigue explícito según metadata: esta política no lo activa ni promete RPO/RTO de producción.

Tras releases exitosos, la retención de ON_DEMAND propios se evalúa contra recibos inmutables y descripción exacta del build. Conserva siete más recientes y al menos siete días; protege el backup actual y no borra backups de fallos, ajenos o históricos. Un fallo de esa limpieza se registra y se difiere, sin revertir un despliegue correcto. La restauración nativa aislada ya ensayada tiene evidencia separada; antes de usuarios reales revisar PITR, retención, RPO/RTO y staging.

Los Jobs efímeros se retiran tras completar/persistir sus resultados; los fallidos o inciertos se conservan para diagnóstico. Imágenes/logs/artefactos siguen su almacenamiento actual. El SQL compartido no se apaga al terminar pruebas.

## Activación vigente

Circuito activo y primer release automático comprobado: build285b7700-7712-4512-ac8a-321c667caa39, fuente293e906d4031a4ed871c1a56c4efdde18ac8a37b, V83/cero pendientes, tres revisiones con tráfico verificado. Bastión, boot disk y reglas exclusivas retirados; lectura privada posterior al borrado PASS. [Recibo operativo](evidence/cierre_tecnico_flujo_inicial.json). El diario contiene los intentos iniciales fallidos antes de SQL y las correcciones TLS/reintentos, sin ocultarlos ni confundirlos con una migración aplicada.
