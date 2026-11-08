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

## Excepción manual: inspección privada de recuperación READ ONLY

El corte [recovery READ ONLY](plan_recovery_readonly_20261107.md) es una ruta manual separada del trigger de main: fuente de rama publicada, archive con generación/hash fijados, imagen con provenance y Job nuevo únicamente con `-recovery-inspection-only`. No ejecuta apply, backup, reparaciones de permisos SQL, unlock ni despliegue. No se presenta como lectura Cloud SQL ya completada mientras no exista reporte actual de ejecución exitosa.

El usuario autorizó después un delta IAM **temporal solo de lectura**, acotado al lock retenido, request exacto de esta inspección y metadata del Job fallido, con revocación/readback obligatorios. Su [plan de alcance y retirada](plan_iam_recovery_readonly_20261107.md) exige conocer primero el parent ID real y su request inmutable, detener el parent antes del provisioner y permitir continuar solo tras recibo de readback operador. No conceder un prefijo general ni editar roles privilegiados existentes. Usar listado/get V2 con parent/nombre de Job/ejecución exactos, no el listado v1 por namespace con filtro de etiquetas.

La herramienta de operador `scripts/gcp/recovery_iam_operator.py` mantiene política/etags y estado de lease privados bajo `.local/recovery-iam/`; `grant` comprueba fuente/parent/lock y crea rol propio con **solo `storage.objects.get`**, dos objetos exactos y expiry acotada, antes de publicar readiness tras readback. `revoke` retira exclusivamente el binding propio con etag actual y comprueba ausencia/retirada del rol; el formato antiguo de tres permisos se acepta solo para retirar una lease propia, nunca concederla. La efectividad real limitada y retirada están documentadas abajo. Ante error/resultado incierto, leer primero lease/políticas; no reejecutar grants, restaurar una política vieja ni volver a lanzar el Job.

**Delta anterior bloqueado, histórico (2026-11-07):** el parent `efabcbe6-8b9f-48f6-9fe4-f38d1ac7aea8` no creó Job ni leyó SQL; su provisioner falló en observación HTTP histórica. Grant retirado, baseline intacto y rol eliminado. No repetir la condición de tres permisos ni ampliar selectores/permisos o habilitar APIs. [Recibo histórico](evidence_recovery_efabcbe6_iam_revoked_20261107.json). Ese intento fue sustituido por la ruta storage-only y la lectura real del registro siguiente; no representa el estado actual de la lectura.

**Ruta storage-only revisada y ensayada (2026-11-07):** request v2 incluye prueba inmutable del parent, fuente, Job/ejecución UID, intent y lock; el parent usa su autoridad existente para observar antes/después de provisionar y aborta antes de ejecutar ante cambio. El provisioner solo crea el inspector, valida prueba de hasta 600 s y lock antes/después, sin lecturas históricas V2. Revisión inicial y 142 pruebas Python/SDK offline en [recibo](evidence_recovery_storage_proof_source_20261107.json); lectura efectiva y retirada abajo. Corrección posterior de limpieza GET y evidencia: **3f6558a37b99575a12aa215a00c1b45b13ded804**, **145 tests Python**, SDK offline y Go focalizado PASS, CI **42429711181 SUCCESS**. No repetir el inspector para fabricar un parent verde ni extender la autorización a otra operación.

## Registro fechado de destinos — lectura de recuperación completada

Fuente comparada **0e62c4ed64a7446c3334ce6bb561aedd0b996426**, inventario V001–V084 inmutable. No confundir la versión del motor con la versión del ledger ni extrapolar una lectura a otro destino.

| Destino | Fecha y fuerza de evidencia | Motor | Ledger/nombres/checksums | Pendientes frente a esa fuente | Gate |
| --- | --- | --- | --- | --- | --- |
| Cloud SQL `espacigo-testprod:southamerica-west1:espacigo-testprod-db`, DB `espacigo_dms_stage_20261020` | **Verificado**, ejecución READ ONLY completada 2026-11-07 **21:15:54 -03** (2026-11-08 **00:15:54Z**) | 18.6 | **83**, prefijo V001–V083 contiguo, nombres/checksums exactos | **V084** | Lectura real cerrada; apply/autoridad DDL **no autorizados ni cerrados** |
| PostgreSQL nativo socket local `5432`, misma denominación histórica de DB | **Último snapshot reportado**, 2026-11-07 17:36:08 -03; no se reconectó en este corte | 18.6 | **76**, V001–V076 exactos al comparar el recibo conservado | V077–V084; V085 candidata pertenece al otro corte, no a esta fuente | Transición runtime LOGIN/configuración y autoridad pendientes |
| Ensayos aislados | Evidencia de pruebas, no destino persistente | Según cada harness | No sustituyen ninguno de los ledgers anteriores | No aplicable | No prueban unificación |

Recibos Cloud: [reporte íntegro de catálogo/ledger](evidence_cloudsql_readonly_report_20261107.json), [ejecución, permisos y retirada](evidence_recovery_storage_operational_20261107.json). `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`, `REPEATABLE READ READ ONLY`. Se observaron **CREATE sobre public=false** y **REFERENCES(usuario.id)=false**; no se modificaron. Estos hechos no bastan para afirmar que sean los únicos bloqueos ni autorizan repararlos.

Parent **8367075d-88d7-492b-8049-c7f006c48120** terminó **FAILURE en comprobación de limpieza**: el inspector completó exitosamente una sola ejecución y publicó reporte validado, pero el listado final requería `run.jobs.list`, no concedido al deployer. **No ampliar IAM ni repetir SQL/Job.** El operador comprobó por GET de nombre exacto que el Job propio ya no existe; recibo remoto `operator-cleanup-readback.json` fijado por generación en la evidencia. La lease storage-only quedó retirada, rol `deleted=true`, baseline semántico de **33 bindings** intacto. Lock histórico generación **1794036269394395** conservado. No presentar el parent como verde ni la corrección local posterior como otro release probado.

Corrección local de limpieza: sustituir listado por GET exacto y aceptar solo el mensaje NotFound de ese nombre del SDK fijado; denied/timeout/otro nombre/Job presente no prueban ausencia. Tres regresiones RED→GREEN, suite **145 Python PASS** y helper real ejecutado contra el Job ya retirado, sin DELETE/list ni nueva ejecución. Revisión pendiente **deleg_f21e42b8** antes de publicar esa corrección. No se aplicó V084, no se alteraron ACL SQL ni se desbloqueó/desplegó aplicación.

Resultado posterior: **deleg_f21e42b8 PASS**, 17 tests independientes y validación del reporte/fuente. [Recibo de revisión](evidence_recovery_cleanup_review_20261107.json). La limpieza exacta y la revocación están cerradas por sus readbacks; el parent histórico conserva FAILURE. No repetir la inspección ni ampliar IAM para cambiar ese estado. El siguiente gate persistente necesita propuesta/autoridad específica para el delta V084 y no queda autorizado por este registro.

## Historia retirada de operación (referencia)

El [archivo anterior a Jobs](cloudsql_migraciones_historial_pre_jobs_20261106.md) conserva diagnósticos, comandos y autorizaciones pasadas para trazabilidad. Quedan sustituidas sus reglas de subir/encender el bastión, abrir un forward SSH, migrar desde local y detener la VM al finalizar. No recrear ese transporte ante un error del pipeline. Los runbooks anteriores de DMS, scratch e importación tampoco sustituyen este procedimiento.
