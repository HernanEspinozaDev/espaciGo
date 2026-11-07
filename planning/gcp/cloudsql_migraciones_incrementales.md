# Cloud SQL — procedimiento vigente de migraciones incrementales

Vigente desde 2026-11-06 por decisión del usuario. Ruta activa: main → Cloud Build → Cloud Run Job privado → Cloud SQL → Cloud Run. El bastión, su disco y reglas exclusivas fueron eliminados; no existe un paso de inicio, SSH o apagado de VM.

## Fuentes operativas

- [Guía de operación y recuperación](../flujo_entornos/operacion_main_sin_bastion.md): procedimiento detallado que debe consultarse antes de una entrega.
- [Skill del flujo](../../.agents/skills/espacigo-release-flow/SKILL.md): preparación, publicación y diagnóstico.
- Implementación: `cloudbuild.yaml`, `scripts/gcp/release_main.py` y `db/release-policy.json`.
- [Recibo de activación y retiro](../flujo_entornos/evidence/cierre_tecnico_flujo_inicial.json): primer release automático SUCCESS, ledger V83 exacto/cero pendientes y conexión privada verificada tras retirar el bastión. Es una lectura fechada; cada release valida su propio estado.

## Secuencia obligatoria

1. Preparar código y SQL en rama temporal desde main; revisar y probar localmente el corte, conservando todas las migraciones aplicadas byte a byte. Una modificación manual local no genera una migración ni se exporta como base completa.
2. SQL nuevo exige archivo versionado y entrada exacta nombre/checksum con `strategy=expand` en `db/release-policy.json`, pruebas fresh/upgrade, compatibilidad con el backend todavía activo y revisión de autoridad DDL para los objetos afectados. No editar ledger, omitir guards ni elevar runtime para resolver drift/permisos.
3. Integrar normalmente a main cuando se cumplan el alcance y los gates del corte; sin PR ni force-push. Publicar main dispara el único trigger, que exige calidad del mismo SHA y clasifica docs/web/full. Docs no toca SQL ni despliega; web verifica que el backend no cambió; el resto usa el circuito completo.
4. El circuito completo toma lock global por generación, construye/verifica imágenes del mismo SHA con provenance y ejecuta plan privado. Verifica backup y solo entonces ejecuta `cmd/dbmigrate` en un Job independiente con conexión privada y secretos montados en runtime. La API no migra y el builder no recibe passwords SQL.
5. Postcheck exige inventario/ledger/checksums sin pendientes y catálogo IMG-01 correcto. Solo después se crean revisiones sin tráfico y se promueve cada revisión verificada, conservando configuración efectiva de los servicios. Persistir evidencia inmutable y resultado; limpiar solo Jobs/lock propios confirmados.
6. Ante fallo o resultado incierto, consultar la ejecución real y los recibos antes de reintentar. No cancelar un Job que podría escribir ni borrar su lock. La recuperación preapply versionada solo libera la generación fallida si prueba ausencia de intención/ejecución aplicadora. SQL no se deshace automáticamente; rollback de aplicación requiere compatibilidad.

## Entorno, permisos y protección

- Única instancia de desarrollo actual: proyecto `espacigo-testprod`, región `southamerica-west1`, instancia `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`. Ese nombre es histórico y no implica un entorno staging. Revalidar metadata/destino en cada release; nunca inferir el estado actual de un recibo antiguo.
- Identidades separadas: ejecutor de release, configurador de Jobs y runtime migrador. SQL usa login dedicado/SET ROLE existentes, locks y transacciones. La delegación del configurador tiene autoridad efectiva DDL; los cambios del pipeline requieren revisión de infraestructura. No conceder superusuario, transferir owners globalmente ni resetear contraseñas por rutina.
- Secret Manager monta la credencial únicamente en runtime del Job. No payloads en URLs, argumentos, logs, commits o archivos compartidos; no reutilizar passfiles históricos.
- Backup sin SQL pendiente: automáticos habilitados con al menos 7 copias COUNT, metadata de hasta 5 minutos y backup exitoso de hasta 24 horas. Con SQL pendiente: ON_DEMAND exitoso iniciado después del lock global. Retención propia: siete más recientes y al menos siete días, con recibos; no borrar backups ajenos/históricos o de fallos.
- PITR sigue deshabilitado según el recibo de activación. Restauración nativa aislada fue ensayada con alcance documentado; no inferir RPO/RTO ni recuperación funcional de producción. Revisar PITR, retención, recuperación y staging al preparar los primeros usuarios.
- Sin develop ni staging durante esta etapa. QA funcional online y aceptación corresponden al usuario; metadata/Ready no las sustituyen.

## Historia retirada de operación

El [archivo anterior a Jobs](cloudsql_migraciones_historial_pre_jobs_20261106.md) conserva diagnósticos, comandos y autorizaciones pasadas para trazabilidad. Quedan sustituidas sus reglas de subir/encender el bastión, abrir un forward SSH, migrar desde local y detener la VM al finalizar. No recrear ese transporte ante un error del pipeline. Los runbooks anteriores de DMS, scratch e importación tampoco sustituyen este procedimiento.
