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

## Registro verificado de esquema — inspección del 2026-11-13

Destino cloud `espacigo-testprod-db/espacigo_dms_stage_20261020`: PostgreSQL18.6, ledger V1–V83 (83 entradas contiguas), nombres/checksums coincidentes con las84 migraciones del SHA `3b39174fef5b173ef5f5c52d1bbaa8d9ed382371`; V84 Didit pendiente. Observación por Job privado REPEATABLE READ READ ONLY, parent `b07742c6-9a5d-4eaa-8b95-1ceec6e47241` SUCCESS; informe generación `1794608791725029`, SHA256 `3de7a3db329652a6cace4f21afc4b715901333b7a919695813f717f2114355d4`. El rol efectivo `espacigo_schema_owner` no tiene CREATE sobre public ni REFERENCES sobre usuario(id). No se concedieron esos privilegios ni se aplicó V84. IAM temporal de lectura retirado/readback; Job inspector ausente; lock fallido conservado. Evidencia operacional: `../frontend_next/evidence/espacios-image-deployment-sync.md`.

La base persistente local y los ensayos aislados no se inspeccionaron en esta operación; no se equiparan con este estado cloud. La inspección no acredita sincronización del despliegue ni autoriza permisos SQL/desbloqueo.

### Excepción acotada de recuperación V84 autorizada después de la inspección

El usuario autorizó separadamente backup válido, CREATE sobre public y REFERENCES sobre usuario(id) temporales sin grant option para schema_owner, aplicación V84 original por Job privado, retirada/readback y recuperación del lock `1794600804441535`. Autorizó además secretAccessor temporal solo en el secreto administrativo existente para `espacigo-test-migrator`, versión1 montada dentro del Job, sin payload local ni acceso a API/builder. El [registro del corte V84](v84_recovery_operation.md) documenta backup `1794610719706` SUCCESSFUL, fuente candidata/revisión, límites y futuras evidencias; no sustituye este procedimiento canónico ni amplía futuras migraciones. El esquema observado sigue V83 hasta readback posterior a la ejecución. Tras la recuperación debe usarse el release normal `release_main.py`, no reconstruir manualmente el despliegue ni omitir sus gates. La base local queda fuera.

## Registro verificado de esquema — V84 aplicada el 2026-11-14 UTC

Destino y motor anteriores confirmados; ejecutor fuente `3c27edbe9f934ed151224073563b62d196705eb4`, inventario84 `3921ff747e4a7e7f49de443de7333dad03535f0687078b93b8a3cac6042a209a`. Job aplicador privado `espacigo-v84-migrate-59763def371744b5-lcrgb` aplicó únicamente V84 original; antes83/pendiente84, después84/cero pendientes. Finalizador `espacigo-v84-adminfinalize-59763def371744b5-cndfm` confirmó count84, autoridad cerrada y baseline `a79501d0b93e6c33c78d46d7f57d5d63cc1e107213aaf2d6b7029e3a6273a041` igual al inicial. Sin cambio de owners, flags ni membresías, y sin grant option.

Postcheck independiente con login migrador/SET ROLE, ejecución `espacigo-v84-inspect-59763def371744b5-hdvn7`: 84 aplicadas/cero pendientes, checksums exactos, `schema_public_create=false`, `usuario_references_id=false`. Catálogo IMG-01 nuevo a `2026-11-14T00:17:43.721983Z` pasó el verificador de producción. Los seis Jobs propios de este intento tienen cleanup/ausencia verificados. El accessor administrativo se retiró; lectura independiente confirmó cero bindings de esa SA sobre el secreto y política semánticamente restaurada. No se descargó el payload ni se dio acceso a API/builder.

Backup1794610719706 conservado y revalidado con metadata fresca antes de prepare y migrate. Recuperación del lock fallido **solo** generación1794600804441535, tras proof fresco y comparación de los84 SQL originales: retirada/ausencia verificadas a `2026-11-14T00:19:42.411514Z`. Release normal nuevo `5ce74175-df7a-4669-b09f-d3a76c6c54dc` del mismo SHA iniciado después; su resultado se registra aparte. Reparación SQL no equivale todavía a despliegue, QA online ni aceptación de producto. Base local intacta.

**Cierre posterior del release:** ese build terminó SUCCESS a `2026-11-14T00:37:23.923059Z`, source3c27edb, con84/cero pendientes y aplicación no-op. API/public/private Ready/100% en revisiones b5ce74175df7a4669b09fd3a7; media env y bucket protegidos releídos. Seguridad final independiente: accessor retirado/política restaurada, ocho Jobs propios ausentes y lock global ausente. No se borraron backups por retención. Fuente/evidencia detallada y límites de QA: `../frontend_next/evidence/espacios-image-deployment-sync.md`. Despliegue técnico completado no significa aceptación funcional.

## Historia retirada de operación

El [archivo anterior a Jobs](cloudsql_migraciones_historial_pre_jobs_20261106.md) conserva diagnósticos, comandos y autorizaciones pasadas para trazabilidad. Quedan sustituidas sus reglas de subir/encender el bastión, abrir un forward SSH, migrar desde local y detener la VM al finalizar. No recrear ese transporte ante un error del pipeline. Los runbooks anteriores de DMS, scratch e importación tampoco sustituyen este procedimiento.
