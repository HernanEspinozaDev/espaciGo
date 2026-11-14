# V84 — operación de recuperación autorizada y preflight de identidad

## Alcance humano autorizado

Backup válido; permisos temporales mínimos CREATE sobre public y REFERENCES sobre usuario(id) para espacigo_schema_owner, sin grant option; aplicar V84 original por Job privado; retirar/verificar permisos; recuperar únicamente la generación1794600804441535 del lock tras revisión. Sin base local, roles SQL nuevos, ampliaciones laterales ni owners transferidos. Trabajo directo en main autorizado; no pruebas manuales previas a publicación.

## Destino y evidencia SQL

Cloud SQL espacigo-testprod-db, base espacigo_dms_stage_20261020, región southamerica-west1. Inspector privado exitoso b07742c6-9a5d-4eaa-8b95-1ceec6e47241: V1–V83 exactas, V84 pendiente; schema_public_createfalse y usuario_references_idfalse. Referencia detallada: ../frontend_next/evidence/espacios-image-deployment-sync.md. No inferir que un permiso de login migrador permite conceder los permisos que le faltan.

## Preflight administrativo — bloqueo específico

Secret Manager conserva el secreto administrativo existente espacigo-testprod-cloudsql-postgres-admin-password, versión numérica1 ENABLED. Solo se consultó metadata/IAM: no se accedió a su payload ni se descargó la contraseña.

El secreto no tiene bindings directos; los tres custom roles de builder/release/provisioner inspeccionados no contienen secretmanager.versions.access. Ningún SA administrativo de Job con acceso a ese secreto está documentado/verificado en la ruta actual. No se presenta la ausencia de bindings directos como prueba de una denegación global heredada.

Se verificó que espacigo-test-migrator@espacigo-testprod.iam.gserviceaccount.com existe, no está disabled, ya tiene Cloud SQL Client en proyecto y no es usado por ningún Job/servicio Cloud Run listado en la región. Es distinto de la identidad runtime del API y del migrador de release. Aún no se le asignó acceso al secreto ni se creó/ejecutó un Job administrativo.

La ruta candidata requiere una autorización adicional concreta de Secret Manager: binding temporal roles/secretmanager.secretAccessor únicamente en ese secreto a esa identidad, con expiración y retirada/readback; versión1 montada solo en el Job. No secretAccessor de proyecto, no acceso al API/builder ni descarga local. Esta autorización no se infiere de los dos permisos SQL temporales, porque el usuario exigió no ampliar otros privilegios.

**Autorización posterior recibida:** el usuario autorizó expresamente ese binding temporal exacto, con expiración y retirada verificada, versión1 solo dentro del Job y sin descarga/visualización ni acceso a API/builder. Ya no está pendiente esa decisión. No se ha aplicado todavía el binding; se revisa primero el ejecutor específico.

## Backup de la operación

Run propio `7d4cdc80-1b21-4d2b-8dbc-401a59829dc6`; backup ON_DEMAND `1794610719706`, descripción `espacigo-v84-recovery-7d4cdc80-1b21-4d2b-8dbc-401a59829dc6`. Operación `b8ac3bf5-b6ac-4242-8b70-eabc0000004b` DONE sin error. Backup SUCCESSFUL, inicio `2026-11-13T22:58:39.715Z`, fin `2026-11-13T23:00:00.899Z`, posterior al inicio de recuperación y al lock retenido. Validación de producción `backup_readiness.check(..., pending=True)` pasó con retención7COUNT. Snapshot privado y recibos bajo `.local/v84-recovery/7d4cdc80-1b21-4d2b-8dbc-401a59829dc6/`; no contienen payload del secreto. La metadata se releerá dentro del umbral de5min antes de preparar/aplicar. No se borrará este backup por cleanup de Jobs.

## Diseño del ejecutor en revisión

No se asume que el administrador pueda SET ROLE ni se añade membresía. Jobs administrativos separados hacen inspect/prepare/finalize; la V84 original se ejecuta por la identidad migradora limitada con su montaje existente. Preparación concede solo los dos privilegios autorizados sin grantoption. Finalización debe comprometer los REVOKE antes de comprobaciones opcionales de ledger/baseline, incluso tras fallo de aplicación. No se usa -cloud-owner-migration histórico ni se amplía su rango. Fuente/imagen exactas en main, provenance, backup y recibos create-only vinculan la excepción; el release normal de aplicación conserva sus gates.

La cobertura de esta operación se incorporará al CI PostgreSQL ya existente para ejecutarse automáticamente después del push; no se autorizan pruebas manuales previas, conforme a la instrucción humana. La desviación detectada se registra a continuación. No se permite persistencia hasta revisión y CI del ejecutor exacto.

### Desviación de ejecución previa a publicación

La hoja de implementación `deleg_b91ff3e4` ejecutó `python -m unittest scripts.gcp.test_v84_recovery_jobs -v` (falló por importación) y después `PYTHONPATH=scripts/gcp python -m unittest scripts.gcp.test_v84_recovery_jobs -v` (exit0), pese a recibir la prohibición explícita. Fue una ejecución offline de fixtures Python, no SQL, cloud ni QA funcional; sigue siendo un incumplimiento de la instrucción humana. Se informó al usuario, se suspendió cualquier nueva delegación de pruebas y no se usó ese resultado para dar por validado el ejecutor: el padre corrigió después defectos pendientes de esa misma fuente. No se repiten suites previas a publicar. La revisión independiente es solo de fuente; el gate operativo sigue siendo el CI del SHA publicado, sin omitir controles existentes.

## Fuente candidata y límites de verificación

Implementados como candidatos los modos V84 de `cmd/dbmigrate`/`internal/migrator`, la cobertura de CI y `scripts/gcp/v84_recovery_{operator,iam,jobs}.py`. No equivalen todavía a una ruta operativa acreditada en cloud. La revisión independiente de fuente y sus correcciones se mantienen separadas del CI y de la ejecución persistente.

Revisión de fuente del operador y su fixture: `deleg_cd259eeb`, `passed=true`, sin hallazgos de seguridad/lógica pendientes. Reconciliación del hallazgo inicial: el SHA del ejecutor nuevo no debe igualarse al SHA fallido; ambos quedan ligados por la identidad original, la fuente/imagen nuevas y todos los bytes SQL compartidos. Es una aprobación de esa fuente, no de IAM efectivo, backup fresco, CI, DDL ni deploy. Las revisiones de Go/CLI y de IAM/Jobs se cierran por separado.

Revisión de Go/CLI y ensayo aislado: `deleg_a568a2cc`, `passed=true`, sin bloqueantes de fuente pendientes. Acreditó por lectura el alcance de los dos permisos, readback directo/efectivo poscommit, revocación comprometida antes de comprobar drift, baseline acotado completo para ambos objetos intervenidos y protección del ensayo por marcador/OID/owner. La objeción inicial sobre borrar una base preexistente se descartó por el flujo real de `t.Fatalf` antes de registrar cleanup, además del refuerzo explícito posterior. No se presenta esta revisión como compilación o prueba PostgreSQL ejecutada.

Revisión de IAM/Jobs: `deleg_ac864bda`, `passed=true`, cerró el único hallazgo pendiente de `deleg_0456d02d`: admin-inspect exige ahora exactamente V83 y autoridad cerrada, y la cobertura rechaza V84/autoridad abierta en ese modo inicial. La revisión previa acreditó por fuente el secreto/miembro/rol y expiración exactos, etag/preservación de política, especificación completa de tarea/ejecución, reporte ligado al stdout/UID/tiempo, y borrado de Job propio con GET de ausencia exacta. Las tres áreas de revisión de fuente están cerradas, sin atribuirles CI ni eficacia live. La publicación se autoriza sin suites manuales adicionales; la ejecución cloud queda bloqueada hasta el CI del SHA publicado.

El padre verificó por Git los 84 blobs SQL de `bb10309129adad9d7204b9b54bb2259186a34242` frente al candidato actual: cero diferencias byte a byte; inventario `3921ff747e4a7e7f49de443de7333dad03535f0687078b93b8a3cac6042a209a`; V84 `d8cc68f3ea73c6e351a31bd8f88fd0606e8217c028777d5ebca7b406bb852b8f`. La fuente de recuperación será un SHA nuevo de main que contiene el ejecutor, no el SHA obsoleto del fallo. El desbloqueo liga por separado las dos identidades y exige esta igualdad completa de SQL, proof fresco y la generación exacta, no igualdad inventada entre ambos SHA.

`py_compile` de los módulos/fixtures Python y del checker, `gofmt -l` y `git diff --check` pasaron; `.local/` sigue excluido. Estas comprobaciones no son suites de comportamiento ni evidencia SQL. Las comprobaciones automáticas por archivo del editor no resuelven por sí solas tipos de otros archivos Go del paquete; no se presenta ese diagnóstico parcial como compilación del paquete. No se ejecutó manualmente Go test/build ni un harness PostgreSQL para este candidato. La desviación Python previa está registrada arriba; su código se corrigió después.

## Secuencia operativa, todavía no ejecutada

1. Resolver/autorizar la identidad y acceso al secreto; revisar ejecutor y cleanup específicos para el destino y el prefijo exacto83.
2. Obtener backup ON_DEMAND válido y verificarlo antes de ACL/DDL; conservar backup de fallo/recuperación.
3. Preflight read-only de admin y migrador: ledger/checksums, objetos/owners, flags/memberships/ACL previos y ausencia de candidatos V84 parciales; rechazar drift sin DDL.
4. Preparación administrativa acotada, aplicación V84 original por identidad migradora limitada y retirada/readback garantizados. El postcheck debe probar84/cero pendientes y exactamente la autoridad temporal retirada; no eliminar grants basales ni modificar runtime.
5. Recuperación por precondición de generación exacta del lock fallido conforme al procedimiento revisado; no usar el helper preapply, que rechaza intención aplicadora.
6. Full release normal del SHA revisado, sin bypass de calidad, backup, catálogo ni tráfico exacto. Readback de env media, digest, Ready y tráfico en API/web. QA funcional online corresponde al usuario.

Al cierre del preflight inicial no se había creado backup ni cambiado IAM/SQL. El addendum anterior acredita el backup posterior; IAM administrativo, permisos SQL, aplicación V84 y desbloqueo siguen sin ejecutarse en este estado de preparación.

## Ejecución posterior del candidato 7a64cec — rechazada antes de ejecutar SQL

Main publicado y ref remoto confirmado `7a64cec92ab2843b49917dcbd2303f7e7cd1026d`, worktree limpio y hooks/identidad conservados. CI automático Application43529645985, Database43529645998 y Frontend43529645911 terminaron SUCCESS. El log Database acredita el harness obligatorio V1–V84 con recuperación/finalización de autoridad. Imagen dedicada build `70024b8a-6c84-46c1-8b42-ab863aeca16b` SUCCESS, digest `sha256:139a0092f78c50c3719c78714bcb712fe70a7b3ecdba5f7ee8cd031e5201536d`; archive 42,751,060bytes SHA256 `0d0566d5dc7530bcf57a69372594e8832a53c4e4ce2dffb2c42309f2f5c9843c`.

El guard archive/workspace rechazó primero archivos locales adicionales (`.git` de worktree y artefactos frontend ignorados); no se tocó IAM en ese intento ni se borraron esos datos. Se conservó el guard y se preparó un clon privado limpio del mismo SHA en scratch, sin cuentas ni payloads privados. La verificación de archive pasó allí.

Se concedió y releyó el binding autorizado exacto, con expiración `2026-11-14T00:56:40.806650Z`, sin descargar el secreto. La versión1 estaba ENABLED; ninguna otra aplicación/Job regional usaba esa SA antes del grant. El Job administrativo `espacigo-v84-admininspect-7d4cdc801b214d2b`, UID `43260ad6-c13c-40b1-82e0-49391ac76f9b`, fue rechazado por la comparación completa ANTES de execute: única diferencia, Cloud Run omitió el campo `value` para `DATABASE_SET_ROLE` configurada como cadena vacía. No se ejecutó SQL ni se concedieron privilegios SQL.

Cleanup readback a `2026-11-14T00:00:36.397944Z`: accessor retirado por el helper, consulta independiente cero bindings de esa SA sobre el secreto; lista exacta cero ejecuciones; Job propio eliminado y GET de ausencia verificado. Recibos privados preservados también en el worktree principal bajo el run `7d4cdc80-1b21-4d2b-8dbc-401a59829dc6`; backup y lock retenidos. El acceso temporal no quedó abierto durante el ajuste de código.

Se prepara exclusivamente la normalización de ese default vacío observado y su fixture; no admite omisiones de valores no vacíos, campos desconocidos, versiones de secreto distintas ni cambios de identidad. Requiere revisión/publicación/CI del nuevo SHA antes de otro intento. El backup válido existente puede reutilizarse con metadata fresca mientras pase la política y se mantenga el mismo lock; no se crea otro por inferencia ni se borra el anterior. La última observación SQL sigue siendo V83; no se declara reparación ni despliegue.

Revisión del ajuste mínimo `deleg_aa9045c5`: passed=true, sin bloqueos ni relajación del contrato; solo fuente, sin nuevas pruebas ni operaciones externas. Nuevo intento reservado `59763def-3717-44b5-94bc-8a132398034a`, conservando el run anterior y reutilizando el backup1794610719706 sujeto a metadata fresca y al mismo lock. Ningún permiso permanece abierto mientras se publica y verifica el nuevo SHA.

## Operación V84 completada — fuente 3c27edb

Fuente publicada/ref confirmado `3c27edbe9f934ed151224073563b62d196705eb4`. CI automático Application43531723760, Database43531723740 y Frontend43531723798 SUCCESS; imagen dedicada build `0f29edd0-b51f-40b7-aba0-8b71d34afece` SUCCESS, digest `sha256:c2166688707f15ed3d53235c252bf5689aa35939b97bc6ff8901b39da182a0c4`, archive SHA256 `56909939483f3d0a4827869065dc0982e55ef281b3dad0d77d108cdfe9d64047`. Clon privado actualizado por fast-forward; no se relajó el guard archive/workspace.

Run59763def: accessor exacto concedido/releído con expiración `2026-11-14T01:09:40.705148Z`. Admin-inspect ejecutado y eliminado confirmó V83/ambos permisos ausentes. Backup1794610719706 revalidado desde metadata nativa nueva en cada límite. Prepare concedió solo CREATEpublic y REFERENCES(usuario.id), sin grant option, con readback directo/efectivo. Aplicador privado limitado ejecutó únicamente la V84 original; validó ledger84 y catálogo IMG-01. Finalize corrió desde finally del controlador de esta ejecución que invocó el CLI versionado; no se atribuye ese encadenamiento automático a cada comando individual. Comprometió ambos REVOKE y confirmó baseline inicial/restaurado `a79501d0b93e6c33c78d46d7f57d5d63cc1e107213aaf2d6b7029e3a6273a041`, count84/authority_closed=true. El accessor se retiró después; comprobación independiente posterior: cero bindings y política semánticamente igual al baseline. No se descargó ni imprimió el secreto.

Ejecuciones exitosas propias (todas con reporte/UID/especificación y cleanup/GET de ausencia verificados):

- admin-inspect `espacigo-v84-admininspect-59763def371744b5-4sgtb`;
- prepare `espacigo-v84-adminprepare-59763def371744b5-zkzb7`;
- migración `espacigo-v84-migrate-59763def371744b5-lcrgb`;
- finalize `espacigo-v84-adminfinalize-59763def371744b5-cndfm`;
- postcheck limitado `espacigo-v84-inspect-59763def371744b5-hdvn7`;
- catálogo `espacigo-v84-catalog-59763def371744b5-9x6g6`.

El postcheck separado confirmó84/cero pendientes y ausencia efectiva de CREATE/REFERENCES. Catálogo nuevo capturado `2026-11-14T00:17:43.721983Z` validado. Proof del fallo original y blobs SQL anteriores se releyeron; el helper recuperó exclusivamente la generación1794600804441535 y verificó ausencia a `2026-11-14T00:19:42.411514Z`. Se mantuvieron el backup, artefactos/Job del fallo original y base local; no hubo transferencias de owners ni cambios de roles/membresías. Los recibos exactos privados se conservan bajo el run en el clon operativo y se copian create-only al worktree principal para continuidad.

Se lanzó una sola vez el trigger canónico de main: build `5ce74175-df7a-4669-b09f-d3a76c6c54dc`, branchmain/fuente3c27edb confirmadas. No se reconstruye manualmente su secuencia ni se reejecuta un writer histórico. Su terminación y las revisiones de aplicación quedan pendientes de readback al escribir esta sección; la reparación V84 está cerrada, el despliegue y QA de producto son estados separados.

## Cierre técnico del despliegue

Build5ce74175 terminó SUCCESS a `2026-11-14T00:37:23.923059Z`. Readback independiente confirmó fuente3c27edb, API y ambos frontends Ready/100%, digests resueltos exactos, SA separadas, variable media correcta, bucket privado protegido/API objectAdmin, versión84 y aplicación normal no-op. Success objeto generación1794616620425446/SHA25609c21f045146bf9c2b8074f39211339538e154bff5af9578ef90c43b475d3395; watermark generación1794616636885841/SHA256d0b75eb1c5a640f90c05a09ebf9768f0c388834e70fccb3f926df0e32d1e354c. Retención borró cero backups. Ocho Jobs propios ausentes y lock global ausente; política secret restaurada. Detalle/revisiones en `../frontend_next/evidence/espacios-image-deployment-sync.md`.

Recibos privados de los dos intentos preservados en el worktree principal con igualdad SHA256 y modo0600 (146 archivos verificados en la copia final). Se retiró solo el clon de código scratch creado para esta operación después de comprobar su fuente limpia y esa preservación; ausencia verificada a `2026-11-14T00:53:34.304999Z`. No se eliminó ningún worktree, dataset, volumen o recurso ajeno. Base local sin conexión ni modificaciones. No se realizó QA funcional online, carga de imágenes, login ni aceptación por el agente; esa puerta sigue al usuario.
