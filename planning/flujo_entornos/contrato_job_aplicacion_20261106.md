# FLUJO-03/05 — contrato del Job aplicador

Estado: implementado y ensayado localmente en la rama de evolución. **No activado en main ni en Cloud SQL.** La ruta cloud operativa conserva el Job de plan READ ONLY registrado en el avance; no se modificó ese Job en este corte.

## Contrato que ejecuta el migrador

El nuevo modo `cmd/dbmigrate -release` requiere:

- SHA completo igual al incorporado al binario al construir la imagen (`SOURCE_SHA` → `main.releaseSourceSHA`). Un binario sin ese valor no puede ejecutar este modo. Este dato no es una firma: el orquestador también debe verificar provenance de la imagen por digest/build/SHA.
- Fingerprint exacto de todos los nombres/checksums del inventario empaquetado y cantidad exacta de migraciones aplicadas revisada en preflight, inicialmente al menos V83.
- Cloud SQL privado `espacigo-testprod:southamerica-west1:espacigo-testprod-db`, base `espacigo_dms_stage_20261020`, login migrador y SET ROLE `espacigo_schema_owner`. Rechaza conexión directa, destinos alternativos, runtime y combinaciones con opciones administrativas/plan/inventario.

En una misma conexión se toma el advisory lock, se inspecciona el ledger en transacción READ ONLY, se comprueba el prefijo revisado, se ejecutan las migraciones pendientes con el runner transaccional existente y se hace postcheck READ ONLY antes de soltar el lock. Los SQL se descubren una sola vez; los bytes en memoria comprobados son los que se ejecutan. La adquisición anidada del runner es reentrante y conserva el lock exterior durante el postcheck.

El resultado `espacigo.migration-execution.v1` incluye contrato, estado anterior/posterior y versiones aplicadas. Solo se emite tras éxito completo y cero pendientes. Un fallo puede dejar archivos anteriores confirmados; no emite éxito y el mismo prefijo antiguo se rechaza al reintentar. Un timeout/resultado incierto requiere inspección, no volver a lanzar SQL por faltar un log.

No se añadieron migraciones, seeds, grants cloud ni repairs. La ruta nueva no habilita bootstrap ni excepciones administrativas. Cada delta futuro necesita revisión de autoridad DDL, compatibilidad anterior/nueva y política de riesgo antes de entrar en la automatización.

## Preparación y comprobación del Job

`scripts/gcp/migration_execution.py` tiene dos acciones **offline**, sin invocar gcloud ni escribir infraestructura:

1. `propose`: valida manifest/preflight y genera arrays de comandos para un Job único por UUID de build, imagen por digest, argumentos de release fijados, una tarea/paralelismo1/reintentos0, Direct VPC y secreto migrador por versión numérica. El lanzamiento usa `--wait` sin overrides. El Job no se crea automáticamente por generar la propuesta.
2. `verify`: recibe readback de la ejecución exacta y logs de stdout obtenidos por el orquestador. Exige nombre/UID del Job creado, proyecto/región/build/SHA, imagen/args/env, cuenta de servicio, red privada y montaje exacto del secreto. Comprueba una única tarea completada sin fallos/reintentos, timestamps recientes y un único reporte estructurado consistente con el preflight y el inventario completo. Un éxito de aceptación del lanzamiento, un log ausente/ajeno o una ejecución de otro Job no habilitan backend.

Ejemplo de generación con archivos revisados, sin ejecutar el Job:

```bash
python3 scripts/gcp/migration_execution.py propose \
  --manifest /ruta/privada/release.json \
  --preflight /ruta/privada/plan.json \
  --build-id UUID-DEL-BUILD-PADRE \
  --secret-version 1
```

Los identificadores son placeholders; las rutas deben contener metadata revisada, nunca payloads de secretos. El verificador no sustituye calidad/provenance/backup, lock global de release ni política de migraciones: son prerrequisitos externos explícitos. No se concedió a Cloud Build acceso a passwords ni autoridad para cambiar Jobs en este corte.

## Pruebas y cierre pendiente

- PostgreSQL desechable: aplicación sintética V1→V2, prefijo/destino/hash incorrectos, no-op revisado, rechazo del reintento antiguo, fallo de V3 después de confirmar V2 y dos releases con el mismo prefijo compitiendo. Solo uno puede completarse; el otro debe detectar prefijo obsoleto.
- Instalación real V1–V83 y actualización V75→V83: nuevo modo de release no-op bajo login migrador/SET ROLE, sin permisos CREATE añadidos para ejecutarlo. El harness exige ahora **21 resultados** y rechaza omisiones/skips.
- Imagen scratch real: inventario de 83 SQL, SHA incorporado correcto y rechazo de SHA distinto/destino local/modos conflictivos, sin red ni credenciales. Build local usa el HEAD como etiqueta de ensayo aun con diff revisado; no se presenta como provenance de una imagen cloud publicada.
- Python: se prueban ejecución válida y rechazo de identidades/imágenes/argumentos/secretos/red alterados, resultados incompletos/antiguos/reintentados y reportes ausentes/duplicados/inconsistentes.

Para cerrar FLUJO-05 falta conectar estos componentes al orquestador activo: adquisición/liberación por generación del lock global, comprobación de backup, construcción/provenance del conjunto de imágenes, provisionamiento separado del Job y captura persistente de su ejecución. También falta producir y verificar desde el Job el catálogo completo exigido por IMG-01; el reporte de ledger nuevo **no reemplaza ese gate existente**. Luego se hará la transición controlada del único trigger a main. No se activa staging en esta etapa.

Referencia oficial consultada: [ejecutar Jobs y esperar finalización](https://docs.cloud.google.com/run/docs/execute/jobs), [consultar la ejecución exacta](https://docs.cloud.google.com/run/docs/managing/job-executions). El patrón aplicado distingue lanzamiento aceptado y ejecución realmente completada.
