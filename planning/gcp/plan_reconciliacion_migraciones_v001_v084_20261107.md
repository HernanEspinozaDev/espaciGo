# Plan de reconciliación de esquema y migraciones V001–V084

**Fecha:** 2026-11-07
**Estado:** auditorías source-only V001–V084 recibidas y matriz canónica reconciliada; follow-up con **0 referencias no calificadas**. V67 y V19 tienen cobertura PostgreSQL específica PASS en harness desechable. La auditoría semántica global sigue **abierta** (`semantic_audit_closed=false`) por revisión parental final, cobertura parcial de V68/V70/V73 y ausencia confirmada de consumidor durable V84. Nativo: último snapshot READ ONLY V001–V076, no renovado. Cloud SQL: lectura recovery READ ONLY V001–V083 exactas/V084 pendiente; sin DDL/grants.

> **Actualización posterior de destino (lectura privada recovery):** Cloud SQL fue inspeccionada en transacción REPEATABLE READ READ ONLY, con ejecución completada el **2026-11-07 21:15:54 -03**. Frente a la fuente V001–V084 en `0e62c4ed64a7446c3334ce6bb561aedd0b996426`, ledger observado: V001–V083 exactas, V084 pendiente; motor18.6, `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`. Autoridad efectiva observada: CREATE en `public` y REFERENCES en `usuario.id` son falsos. No se concedieron. El parent recovery acabó FAILURE solo en el chequeo de limpieza por `run.jobs.list`; inspector SUCCESS, Job exacto ausente por GET, IAM temporal retirado/readback y lock original conservado. Recibos copiados aquí: [`reporte SQL`](evidence_cloudsql_readonly_report_20261107.json), [`resultado operacional`](evidence_recovery_storage_operational_20261107.json), [`revisión de cleanup`](evidence_recovery_cleanup_review_20261107.json). Esta actualización reemplaza toda declaración anterior de “Cloud ledger no leído/no verificado”; no cierra aplicación SQL, autoridad global, release ni auditoría semántica.
**Límite de KYC:** el contrato/código puede continuar en rama aislada con pruebas sintéticas; el uso de persistencia real, migración o recorrido integrado que dependa de V84/V85 queda bloqueado hasta cerrar las puertas de reconciliación descritas aquí.

**Actualización V84, 2026-11-07 tarde:** gate de código/ensayo local verificado con PostgreSQL desechable: 56 resultados obligatorios sin skips, fresh V1→V84 y upgrades V75/V76→V84, autoridad acotada/memberships/ACL/defaults y release V83→V84. Se corrigió el lector de logs que descartaba `ddl_authority`. Esto no verifica autoridad/ledger Cloud SQL ni cierra la auditoría independiente 84/84. Detalle y exclusiones: `evidencia_autoridad_ddl_v84_20261107.md`. La sección 2 conserva el snapshot inicial anterior al ensayo; no usar sus referencias a pruebas aún no ejecutadas como estado actual.

## 1. Objetivo y alcance

Revisar, en orden y sin saltos, el historial versionado `V000001`–`V000084`, los contratos de persistencia, las autoridades de ejecución y cada ledger de destino por separado. Resolver los conflictos con cambios versionados, pruebas y evidencia; no deducir el estado actual de una base a partir del código, de un recibo antiguo ni de otra base.

La versión del esquema significa el máximo prefijo verificado de `schema_migrations`; no es la versión del motor PostgreSQL ni la versión desplegada de la aplicación. La comprobación de las 84 migraciones del repositorio no prueba que una base haya aplicado esas 84.

## 2. Estado inicial comprobado y límites

- El inventario local contiene **84 archivos**, versiones consecutivas 1–84, sin huecos ni duplicados. Se extrajo la lista exacta de nombres en la sección 8.
- `db/release-policy.json` declara baseline 83 y registra `V000084__didit_identity_verification_schema.sql` como `expand`, con checksum `d8cc68f3ea73c6e351a31bd8f88fd0606e8217c028777d5ebca7b406bb852b8f`.
- El runner aplica cada archivo en una transacción propia y registra su fila del ledger dentro de la misma transacción (`internal/migrator/runner.go:312–331`). Por eso la tabla temporal `ON COMMIT DROP` de V057 vive durante todo su SQL y se elimina al cerrar esa migración; no es por sí sola una incompatibilidad. Esta atomicidad no cubre grants/revokes ejecutados por operaciones administrativas separadas del runner.
- El recibo de activación del 2026-11-06 era histórico. **Fue supersedido como evidencia del ledger por la lectura READ ONLY recovery del 2026-11-07 21:15:54 -03**, V1–V83 exactas/V84 pending; esa lectura es un snapshot fechado, no estado perpetuo.
- El harness aplica V001–V080 por la conexión administrativa sin `SET ROLE` (`internal/migrator/current_schema_integration_test.go:61–71`); después concede temporalmente `CREATE ON SCHEMA public` y `REFERENCES ON public.usuario` a `espacigo_schema_owner` para V84 y los revoca (`:100–107`). Por ello, el `usuario` creado por ese harness no representa necesariamente su owner en el Job de release. El Job configura `DATABASE_SET_ROLE=espacigo_schema_owner` (`scripts/gcp/migration_execution.py:22–31`, `scripts/gcp/release_jobs.py:23–29`). V84 crea tablas y una FK a `public.usuario(id)` (`db/migrations/V000084__didit_identity_verification_schema.sql:3–5`); `CREATE`/`REFERENCES` deben evaluarse como privilegios efectivos, considerando owner y ACL, no solo grants explícitos. La orquestación valida el ledger/policy pero no informa ni verifica esos privilegios: el contrato del plan admite solo `database`, `server_version`, `read_only`, `history_exists`, `applied` y `pending` (`scripts/gcp/migration_release_manifest.py:32–35`), y `migration_policy` compara nombre/checksum/estrategia (`scripts/gcp/release_main.py:140–145`). Hay bootstrap fuente que concede `CREATE`, pero está limitado a la base `espacigo` y al actor bootstrap (`db/bootstrap/gcp04-runtime/005_cloud_bootstrap.sql:9–11`); la normalización local de owners también rechaza otra base (`db/bootstrap/gcp04-runtime/003_local_source_owner_normalization.sql:12–13`). Ninguno prueba por sí solo el estado de `espacigo_dms_stage_20261020`. **Conclusión actualizada:** el plan de release no hace readback de autoridad DDL; la inspección recovery confirma que el actor observado carece de `CREATE public` y `REFERENCES(usuario.id)`. El error V84 `42501 permission denied for schema public` es consistente con la falta de CREATE; no se afirma que REFERENCES fuera el punto del fallo ni que se haya reparado. No conceder grants sin delta mínimo revisado y autorización.
- Se recibieron cuatro auditorías source-only de V001–V020, V021–V040, V041–V060 y V061–V084 en el checkout/HEAD fijados arriba. Esos reportes no ejecutaron tests/SQL ni consultaron bases. La revisión parental corrigió claims críticos de V64/V66/V68/V70/V84 y sigue verificando referencias y cobertura; las 84 filas no equivalen a cierre semántico mientras queden referencias o dictámenes sin aceptación.
- `planning/gcp/migraciones_source_manifest_v001_v084_20261107.tsv` fija nombre y SHA-256 de cada archivo fuente en el HEAD de trabajo; su raíz es `160b647b738ae04300803b1f690adf33cd54348ec29a1a55d8db7a8acd56d51f`. Verificado: 84 filas consecutivas y todos los hashes coinciden con los bytes actuales. Es un baseline de código, no un recibo de ledger aplicado.
- Hay más fronteras de autoridad que V84: V009 y V018 usan `CREATE EXTENSION` (`btree_gist`, `postgis`); V060, V064 y V066 crean roles/conceden permisos de base o documentan roles; V081 exige roles/schema/grants preprovisionados; V082–V083 requieren transiciones acotadas de owner/`CREATE`; V084 requiere `CREATE` y `REFERENCES` efectivos para su DDL si no se satisfacen por owner/ACL. Esos requisitos de fuente no demuestran privilegios efectivos de Cloud SQL.
- El render/aplicación del release vigente rechaza ledgers con menos de V83 (`scripts/gcp/migration_execution.py:16–19`). Por tanto, el release actual **no es** un instalador fresco V001→V084. El harness sintético sí separa la aplicación V001–V080 mediante una identidad administrativa, preprovisiona roles para V81 y abre/revoca ventanas para V82–V84 (`internal/migrator/current_schema_integration_test.go:45–107`). Estas rutas se deben evaluar separadamente.
- V84 admite columnas de identidad declarada, JSONB extraído y payload webhook, y URL de sesión en campos sin cifrado de aplicación (`db/migrations/V000084__didit_identity_verification_schema.sql:10–22, 41–52`). El plan de KYC exige protección de identidad/resultados y minimización del inbox (`planning/kyc_didit/plan_kyc_didit.md:96–102`). El handler KYC sigue en memoria; la existencia de V84 no demuestra persistencia funcional.
- El árbol de trabajo ya contiene cambios KYC locales no confirmados. Se preservan; este corte no los revierte, integra, publica ni modifica SQL aplicado. Tras `git fetch origin --prune`, `origin/main=5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`; `main` local=`75c8c21d30c70a4fd63f6109664d5c2a063410df`. No hay diferencias de `db/migrations` ni `db/release-policy.json` entre candidato y `origin/main` o `main` local.
- El preflight READ ONLY local del 2026-11-07 verificó target `127.0.0.1:5432/espacigo_dms_stage_20261020`, PostgreSQL 18.6, DB OID 16388, `session_user=current_user=postgres`; ledger V001–V076 continuo (76 filas), nombres/checksums coincidentes con el candidato y V077–V084 pendientes. Un snapshot posterior de catálogos halló `usuario` y `schema_migrations` owned por `postgres`, schema `public` owned por `pg_database_owner`, roles `espacigo_schema_owner` y `espacigo_migrator_login` ausentes, y `espacigo_runtime` como LOGIN sin memberships. No extrapolar esos datos locales a Cloud SQL. Evidencia: `planning/gcp/evidence_ledger_preflight_20261107.md`.
- En la lectura recovery posterior no se ejecutó DDL, no se concedieron/revocaron privilegios ni se hicieron backups/despliegues. Cloud SQL se consultó en READ ONLY y su ledger V1–V83/V84 pending quedó registrado en los recibos posteriores; no sustituye una lectura fresca para una aplicación futura. Los ensayos PostgreSQL desechables y su cleanup se documentan por separado.

## 3. Reglas de seguridad e invariantes

1. Consultar `planning/gcp/cloudsql_migraciones_incrementales.md` y `planning/flujo_entornos/operacion_main_sin_bastion.md` antes de cada operación de base o release. Cloud SQL se opera exclusivamente por el Job privado vigente; no recrear bastión ni abrir túnel local.
2. Mantener cada destino como registro separado: persistente nativo, Cloud SQL y bases sintéticas. No copiar datos de una base a otra ni usar una para inferir la versión de otra.
3. Antes de DDL: destino e identidad verificados, ledger/checksums/owners preflight de solo lectura, plan revisado, pruebas del delta y backup válido según el runbook. No realizar DDL en esta revisión de planificación.
4. No editar `schema_migrations`, saltar versiones, ejecutar SQL manual fuera del migrador versionado, elevar `espacigo_runtime` ni conceder grants globales/superusuario.
5. Preservar byte por byte toda migración aplicada en cualquier destino. Antes de decidir si V84 u otra migración pendiente puede corregirse en su mismo archivo, reconciliar todos los ledgers y obtener revisión/autorización del cambio de checksum. Si una versión ya fue aplicada, corregir hacia adelante con una migración nueva; nunca reescribir historia.
6. Grants temporales solo si se demuestra que son imprescindibles, con principal/objeto/versión acotados, expiración operacional y revocación garantizada incluso ante error, cancelación o terminación; leer de vuelta el privilegio efectivo después. No ocultar la brecha agregando permisos persistentes amplios.
7. Separar evidencia: análisis estático, unit tests, PostgreSQL sintético, ledger local persistente, ledger Cloud SQL, despliegue y aceptación de producto son gates distintos. Un test omitido no cuenta como PASS.

## 4. Fases, secuencia y gates

### Fase 0 — Congelar el contexto y verificar el inventario

- [x] Preservar la rama y cambios locales existentes; rama actual `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD observado `274f37e483a078890e92382a159fa49fd23c2fe8`.
- [x] Contar archivos, validar nombres, versiones 1–84 consecutivas, sin duplicados ni huecos.
- [x] Registrar baseline/policy 83→84 y el último recibo conocido como histórico, no actual.
- [x] Fijar candidato: worktree sucio preservado, rama/HEAD completo confirmados, `origin/main` actualizado y comparados `db/migrations`/`db/release-policy.json` contra candidato y `main` local; sin diferencias en esos paths.

**Gate:** inventario reproducible y alcance de datos preservado. La verificación de fuente no sustituye un ledger fresco.

### Fase 1 — Auditoría secuencial V001–V084

Para **cada versión**, en orden, completar la casilla individual de la sección 8 y registrar evidencia con:

- checksum SHA-256 en el SHA candidato y si está registrada en `db/release-policy.json` cuando corresponda;
- DDL, objetos, owner, dependencias FK/check/trigger/function/extension y efecto de datos/backfill;
- autoridad requerida por sentencia y transición de roles antes/después (incluyendo `CREATE`, `REFERENCES`, comentarios de roles, grants de base/esquema y `SET ROLE`);
- comportamiento de transacción, repetición, rollback y orden relativo; cualquier irreversibilidad;
- contrato consumidor real: queries/repositorios Go, DTO/OpenAPI, retención/privacidad, grants runtime y procesos/workers;
- pruebas fresh install, upgrade desde prefijo anterior, pruebas PostgreSQL pertinentes, harnesses, conteos y evidencia de ejecución;
- compatibilidad con la aplicación desplegada y con versiones aplicadas en destinos conocidos;
- hallazgos, severidad, estado y decisión: sin conflicto, corregible en código, migración correctiva necesaria, evidencia/decisión pendiente o fuera de alcance.

**Regla:** no marcar una versión resuelta solo porque el SQL parsea o el test de fuente pasa. Si una migración tiene relación con otra, registrar ambos lados y no cerrar la primera hasta probar la transición conjunta.

### Fase 2 — Reconciliación cruzada del runner y release

- [ ] Trazar `cmd/dbmigrate`, `internal/migrator`, guard de historial, advisory lock, inicialización del ledger y checksum contra el flujo de release completo.
- [ ] Construir tabla de autoridad por frontera: extensiones V009/V018; role/DB grants V060/V064/V066; prefijos 1–80; V81 (roles/helpers/ledger preprovisionados); V82–V83 (CREATE/replace y ACL); V84 (CREATE de tablas y REFERENCES a `usuario`). Validar cada transición y separar fresh-install sintético del release V83→V84.
- [x] Comparación fuente del harness y el Job: V001–V080 se instalan como admin en el harness; V81–V84 usan `SET ROLE espacigo_schema_owner`; el Job de release fija el mismo `SET ROLE`. El pregrant de `REFERENCES` del harness no demuestra que Cloud SQL lo necesite.
- [x] Contrastar `session_user/current_user`, ledger y privilegios requeridos por V84 en Cloud SQL mediante inspección privada READ ONLY; lectura V83/V84 pendiente y CREATE/REFERENCES falsos. No conceder privilegios por inferencia.
- [x] Ejecutado el harness PostgreSQL aislado: rechazo sin ambos grants, solo CREATE y solo REFERENCES; SQLSTATE `42501`/objeto comprobados separadamente mediante rollback sintético. Los rechazos conservan V83/sin objetos V84; aplicación con CREATE y REFERENCES(id), revocación y readback PASS.
- [x] Ruta Job/runner revisada: no crea privilegios DDL por un canal paralelo. Pregrants del harness explícitos y reducidos a REFERENCES(id); no hay bootstrap/revoke V84 en el Job. Una ventana administrativa persistente sigue sin autorización ni implementación y no se infiere de la prueba.
- [x] Reporte read-only `ddl_authority` ampliado a owners, identidad, esquemas efectivos, flags/memberships de los tres roles y ACL directas/PUBLIC/columnas/defaults de tablas en el alcance V84; revalidación en transacción y evidencia transaccional en reporte de release. No es auditoría global de ACL ni prueba de ausencia de privilegios fuera de ese alcance. Ver [`evidencia_autoridad_ddl_v84_20261107.md`](evidencia_autoridad_ddl_v84_20261107.md).
- [x] Gate local: 56 resultados PostgreSQL obligatorios sin skips, tests Go con race y vet, y suites Python offline PASS. `release_jobs` ya preserva el nuevo reporte y valida autoridad; la prueba de wire real Go→JSON→Python pasa. Cloud SQL no se consultó: su gate de autoridad efectiva permanece abierto.

**Gate:** matriz de autoridad y mecanismo de revocación revisados; no se activa ningún grant en destino persistente por esta fase.

### Fase 3 — Reconciliar cada destino con lectura autorizada

Para cada destino, emitir una instantánea con host/puerto/instancia/base, motor, `session_user`, `current_user` tras `SET ROLE`, hora, SHA de fuente, versiones aplicadas, nombres/checksums, prefijo continuo, pendientes, owners/ACL relevantes y evidencia exacta.

- [x] PostgreSQL nativo persistente: preflight peer READ ONLY; V001–V076 continuo y coincidente con el candidato, V077–V084 pendientes. Snapshot de autoridad limitado: owner `postgres` para ledger/`usuario`; `schema_owner` y `migrator_login` ausentes; `runtime` LOGIN, `CONNECT`/`USAGE` y DML consultado sobre `usuario`, sin CREATE en `public` ni SELECT del ledger. Evidencia en `planning/gcp/evidence_ledger_preflight_20261107.md`; no se arrancó API ni worker.
- [x] Cloud SQL: inspección privada recovery READ ONLY y checksum exacto V1–V83; V84 pendiente; autoridad requerida reportada. La inspección no fue el modo `preflight-only` del release y no ejecutó backup/apply/rollout.
- [ ] Cada base de ensayo/demo retenida que el usuario autorice: inventariarla por separado; preservar volumen `espacigo_pgdata`, datos y credenciales ajenos.
- [ ] Reconciliar `main`, candidato y cualquier checkout activo mediante nombres/checksums, distinguiendo archivos presentes de versiones aplicadas.

**Gate:** no quedan destinos mezclados o inferidos. Si un destino no se puede leer por el camino autorizado, queda explícitamente sin verificar; no se sustituye por otra base.

### Fase 4 — Reproducción PostgreSQL aislada

Solo sobre un destino sintético con nombre, endpoint, imagen y alcance autorizados; registrar su aislamiento y limpieza antes de iniciar.

- [x] Fresh install sintético V001→V084 por etapas explícitas en el harness aislado; no es capacidad del Job ni autoriza bootstrap persistente.
- [x] Upgrades sintéticos V75/V76→V84 por etapas y release V83→V84 con login/SET ROLE del Job; prefijos conservados, pregrants explícitos, no-op y contrato API/catálogo PASS.
- [ ] Probar de forma independiente los límites de autoridad de V009/V018, V060/V064/V066 y la precondición de roles de V081; registrar qué operación requiere admin/bootstrap y cuál ejecuta `espacigo_schema_owner`.
- [x] Casos V84 sin CREATE y sin REFERENCES por separado: SQLSTATE/objeto observados; aplicación con REFERENCES(id), revocación y readback final PASS, únicamente sintético.
- [ ] Probar interrupción/fallo entre grant, migración y revoke; demostrar cleanup incluso si la ejecución de migración falla.
- [ ] Ejecutar verificaciones de relaciones, owners, ACLs, columnas, triggers/functions, checks, índices, ledger/checksum, y regresión de consultas Go para las versiones afectadas.
- [ ] Probar la forma corregida de almacenamiento sensible KYC en la capa DB solo después de definir el contrato criptográfico y sus límites; no insertar identidades reales ni persistir payloads Didit reales.

**Gate:** test fresco y upgrade reproducibles, sin skips que cubran requisitos; cleanup leído desde conexión independiente. Esta fase no es permiso para alterar DB persistente ni Cloud SQL.

### Fase 5 — Resolver conflictos con cambios mínimos y forward-only

- [ ] Clasificar cada hallazgo y decidir si se corrige en SQL aún no aplicado en ningún destino o en nueva migración; revisar el ledger de todos los destinos antes de decidir.
- [ ] No editar migraciones aplicadas. Conservar checksums e historial; una reparación de esquema aplicada requiere SQL versionado, probado y forward-only, nunca editar el ledger.
- [ ] Resolver primero la autoridad necesaria para aplicar la cadena existente; no añadir permiso a `espacigo_runtime` para que el migrador pase.
- [ ] Resolver V84/privacidad: no almacenar identidad legal/RUT, extracción sensible o payload de webhook indiscriminadamente en claro; separar declaración cifrada, intentos y un inbox de eventos mínimo. Cifrar antes de persistir. Revisar índices/retención/borrado y claves/rotación sin escribir valores de claves en repositorio.
- [ ] Registrar cada cambio en policy con nombre/checksum exactos, estrategia compatible, pruebas fresh/upgrade y revisión local. No crear una V85 hasta demostrar que es necesaria y que la secuencia pendiente se ejecutará íntegra.
- [ ] Actualizar harnesses, pruebas, runbook/evidencia y conteos de manera consistente, sin presentar propuestas como capacidades instaladas.

**Gate:** cada conflicto tiene corrección, prueba, checksum y autorización de destino apropiados; el diff no reescribe historia.

### Fase 6 — Cierre de reconciliación y desbloqueo de KYC

- [ ] Revisar los 84 ítems de la sección 8; todos deben tener evidencia o excepción aprobada explícita.
- [ ] Probar árbol limpio respecto a migraciones aplicadas, inventario/policy coincidente, tests y `git diff --check`.
- [ ] Actualizar el registro de versión por destino con fecha y evidencias; comprobar estado del ledger después de cualquier release autorizado y dejar separado el despliegue de aplicación.
- [ ] Confirmar que no hay conflicto de esquema/autoridad/contrato de persistencia que impida KYC y que V84 no se presenta como persistencia funcional mientras el handler no use repositorios durables.
- [ ] Mantener bloqueados el DDL y la integración KYC contra destinos persistentes hasta cerrar este gate; el desarrollo Go/OpenAPI y sus pruebas aisladas puede continuar con datos sintéticos autorizados, sin declarar persistencia real. Para recorrido integrado, respetar orden DB → Go/OpenAPI → frontend → evidencia.

**Gate final:** KYC continúa solo con reconciliación local verificada y límites de destino claros. La aceptación de producto y QA online siguen perteneciendo al usuario.

## 5. Conflictos ya identificados (no es inventario exhaustivo)

1. **Autoridad V84:** omisión de código resuelta y probada localmente: reporte y gate dentro de la transacción, perfiles de ownership explícitos, memberships exactas y ACL/PUBLIC/defaults del alcance V84. El reporte recovery ahora confirma para el actor observado `CREATE public=false` y `REFERENCES(usuario.id)=false`; no se cambió autoridad. Harness y wire Go→Python PASS; lector Job conserva el reporte. El Job no crea/revoca los pregrants del fixture. La lectura no prueba mínimo privilegio global ni identifica por sí sola la reparación aprobada.
2. **`REFERENCES` y ownership:** V84 referencia `public.usuario(id)`. La lectura recovery confirma que el `current_user=espacigo_schema_owner` observado carece de CREATE efectivo en `public` y REFERENCES efectivo en `usuario.id`; este es un gate real de Cloud SQL para aplicar esa cadena, distinto de la snapshot nativa. No añadir/grant por inferencia: diseñar el alcance mínimo y su procedimiento de readback/revocación, con autorización exacta; no confundir test negativo/positivo sintético con reparación persistente.
3. **Contrato de privacidad V84:** el DDL permite datos personales y payload JSONB en claro, distinto de la exigencia de cifrado/minimización del plan KYC. Cloud ledger reporta V84 pendiente, pero la inspección no leyó filas de negocio ni demuestra que no haya objetos/estado parcial fuera del ledger. No usar las tablas hasta revisar el estado/consumidores y el contrato V85; no backfill ni migración hasta gates y autorización separados.
4. **Persistencia faltante:** el mapa en memoria del handler no usa `identity_verification_attempts`/`didit_webhook_inbox`; esto es discrepancia entre esquema y flujo, no un error que se arregle con grants.
5. **Estado por destino:** Cloud SQL fue leído en recovery: V001–V083 exactas/V084 pendiente, `CREATE public=false`, `REFERENCES(usuario.id)=false`; el reporte es acotado y no prueba mínimo privilegio global. Nativo sigue en snapshot histórico READ ONLY V001–V076 con gates propios; no trasladar hechos entre destinos.
6. **Fresh install no equivale a release:** el release actual exige prefijo aplicado ≥V83; los cortes anteriores incluyen extensiones y creación/grants de roles que el migrador restringido no puede asumir por defecto. Debe quedar definido y probado el camino staged bootstrap, sin elevar permanentemente `espacigo_schema_owner` ni convertir la prueba administrativa en un bypass de producción.
7. **V014 — orden total no demostrado:** la migración ordena por fecha, estados y motivo, pero no completa con un campo único estable (`db/migrations/V000014__m06_deterministic_history_backfill.sql:6–20`); V011 solo garantiza UUID único y no unicidad de esa tupla (`db/migrations/V000011__m06_local_booking_trial.sql:63–72`). La fuente permite empates legales, para los que `row_number()` puede asignar distinto orden; que existan empates en datos de destino sigue sin verificarse. Antes de proponer reparación, comprobar el contrato de escritura y los datos por destino; si V014 ya está aplicado, no editarlo.
8. **V015 — precondición de datos pendiente:** el backfill de cotizaciones usa `JOIN espacio_caracteristicas` y luego hace `NOT NULL` los snapshots (`db/migrations/V000015__m05_local_synthetic_catalog.sql:16–27`). Si existe una cotización sin fila de características, la migración falla; no se comprobó que exista ese caso ni que el API lo permita. Reconciliar con validación de datos read-only autorizada o prueba sintética de migración.
9. **V079 — ACL de lectura por revisar:** concede `SELECT` de tabla a runtime para caso, archivo y reviewer grant (`db/migrations/V000079__document_verification_cases.sql:163–173`), además de grants por columna de escritura. La amplitud real frente al DTO/API depende de columnas consumidas; es una hipótesis de mínimo privilegio, no una brecha confirmada. Comparar consultas Go y política antes de cambiar ACL.

## 6. Evidencia y formato de actualización

Mantener el plan como checklist maestro. Por cada fase registrar fecha, checkout/SHA, archivos fuente, comandos reales, resultados, skips, destinos/identidades (sin secretos), checksums, permisos y límites. Evidencia de base debe ser independiente del plan: recibos con ledger leído en transacción de solo lectura o salida del Job real; no incluir contraseñas, URLs con credenciales, datos personales, tokens ni archivos privados. La evidencia del gate de código V84 está en [`evidencia_autoridad_ddl_v84_20261107.md`](evidencia_autoridad_ddl_v84_20261107.md) y no equivale a evidencia de Cloud SQL.

Formato de resultado por versión:

`Vnnn | estado | checksum/fuente | objetos y dependencias | autoridad | consumidores | pruebas | destinos con ledger verificado | conflicto y corrección | evidencia`.

Estados permitidos: `pendiente`, `revisada-sin-conflicto`, `conflicto-confirmado`, `corrección-en-curso`, `resuelta-con-evidencia`, `bloqueada-por-acceso/decisión`. No usar `resuelta` si solo se inspeccionó el SQL.

## 7. Registro por destino (no inferir ni combinar)

| Destino | Último dato disponible | Estado al 2026-11-07 | Acción de reconciliación |
|---|---|---|---|
| Cloud SQL `espacigo-testprod-db` / `espacigo_dms_stage_20261020` | Inspección privada READ ONLY terminada 2026-11-07 21:15:54 -03; `evidence_cloudsql_readonly_report_20261107.json`, source `0e62c4ed64a7446c3334ce6bb561aedd0b996426` | V001–V083 exactas; V084 pendiente; autoridad efectiva `CREATE public=false`, `REFERENCES usuario.id=false`. Parent cleanup FAILURE separado de inspector SUCCESS; lock retenido e IAM retirado/readback. | Revisión de autoridad/compatibilidad V84/V85, plan/backup y autorización antes de DDL; no repetir recovery ni ampliar IAM |
| PostgreSQL nativo local | 2026-11-07 08:03:45-03 ledger y 08:10:56–08:11:33-03 catálogos; `127.0.0.1:5432/espacigo_dms_stage_20261020`, PostgreSQL 18.6, DB OID 16388, peer `postgres` | Ledger V001–V076 continuo y coincidente; V077–V084 pendientes. Owner de `usuario` y ledger: `postgres`; owner de `public`: `pg_database_owner`; `espacigo_schema_owner`/`espacigo_migrator_login` ausentes; `espacigo_runtime` es LOGIN, sin memberships. Efectivos consultados para runtime: CONNECT/USAGE/usuario SELECT+UPDATE sí; CREATE public y ledger SELECT no. | Evidencia `planning/gcp/evidence_ledger_preflight_20261107.md`; snapshot acotado, no aplicar a Cloud SQL; sin DDL ni cambios ACL |
| Demos/volúmenes retenidos | No consultados | Preservados, fuera de alcance hasta autorización específica | No arrancar, mutar, limpiar ni inferir su ledger por semejanza |
| Base sintética para ensayos | Harness sin red/puertos/volúmenes, imagen PostGIS 18 fijada, socket privado y PGDATA tmpfs; último contenedor `espacigo-migration-ci-1468668` | V84 fresh y upgrades V75/V76 comprobados; 56 resultados requeridos PASS, contenedor propio retirado y ausencia verificada | Evidencia `evidencia_autoridad_ddl_v84_20261107.md`; no sustituye ledger nativo/Cloud SQL |

## 8. Checklist completo, en orden de ejecución

Marcar una versión únicamente al completar la revisión de Fase 1 y anotar evidencia en el registro. Se recibieron reportes source-only para las 84 versiones el 2026-11-07; aún falta contrastarlos con el padre y cerrar dependencias, autoridad, consumidores y pruebas requeridas. Por eso todas las casillas siguen pendientes; ningún reporte afirma estado de ledger real. V084 tiene conflictos iniciales confirmados a nivel de fuente, pero continúa sin cerrar.

### V001–V010
- [ ] V001 — `V000001__m01_identity.sql`
- [ ] V002 — `V000002__m02_profile_rights.sql`
- [ ] V003 — `V000003__m03_verification.sql`
- [ ] V004 — `V000004__m03_reason_codes.sql`
- [ ] V005 — `V000005__m04_space_drafts.sql`
- [ ] V006 — `V000006__m04_extensible_category_attributes.sql`
- [ ] V007 — `V000007__m04_profile_attribute_order.sql`
- [ ] V008 — `V000008__m03_synthetic_evidence.sql`
- [ ] V009 — `V000009__m06_draft_manual_occupancy.sql`
- [ ] V010 — `V000010__m05_versioned_rates_and_private_simulations.sql`

### V011–V020
- [ ] V011 — `V000011__m06_local_booking_trial.sql`
- [ ] V012 — `V000012__m06_local_booking_conditions_snapshot.sql`
- [ ] V013 — `V000013__m06_booking_history_sequence.sql`
- [ ] V014 — `V000014__m06_deterministic_history_backfill.sql`
- [ ] V015 — `V000015__m05_local_synthetic_catalog.sql`
- [ ] V016 — `V000016__m09_local_reservation_messages.sql`
- [ ] V017 — `V000017__m09_reservation_message_read_cursor.sql`
- [ ] V018 — `V000018__m05_synthetic_fixture_locations.sql`
- [ ] V019 — `V000019__m06_local_weekly_hours.sql`
- [ ] V020 — `V000020__m06_local_flexible_cancellation.sql`

### V021–V030
- [ ] V021 — `V000021__m06_local_payment_inbox.sql`
- [ ] V022 — `V000022__local_identity_history_and_outbox.sql`
- [ ] V023 — `V000023__local_privacy_suppression_review.sql`
- [ ] V024 — `V000024__local_dispute_privacy_blocker.sql`
- [ ] V025 — `V000025__local_synthetic_privacy_suppression.sql`
- [ ] V026 — `V000026__local_privacy_retention_purge_and_replay.sql`
- [ ] V027 — `V000027__local_credential_notice_terminal_recovery.sql`
- [ ] V028 — `V000028__local_kyc_eligibility_and_history.sql`
- [ ] V029 — `V000029__local_privacy_kyc_eligibility.sql`
- [ ] V030 — `V000030__m02_synthetic_profile_photo_and_payout.sql`

### V031–V040
- [ ] V031 — `V000031__m04_local_publication_lifecycle.sql`
- [ ] V032 — `V000032__m04_synthetic_space_gallery.sql`
- [ ] V033 — `V000033__m04_gallery_candidate_cleanup.sql`
- [ ] V034 — `V000034__local_synthetic_contract_signing.sql`
- [ ] V035 — `V000035__local_synthetic_rental_operations.sql`
- [ ] V036 — `V000036__local_communication_reviews_and_notices.sql`
- [ ] V037 — `V000037__local_communication_privacy_coordination.sql`
- [ ] V038 — `V000038__local_damage_claim_resolution.sql`
- [ ] V039 — `V000039__local_damage_claim_privacy_retention.sql`
- [ ] V040 — `V000040__local_guarantee_and_dispute_finance.sql`

### V041–V050
- [ ] V041 — `V000041__local_admin_account_block.sql`
- [ ] V042 — `V000042__admin_block_privacy_minimization.sql`
- [ ] V043 — `V000043__local_admin_financial_audit_actions.sql`
- [ ] V044 — `V000044__local_fake_settlement_receipt.sql`
- [ ] V045 — `V000045__local_settlement_privacy_obligations.sql`
- [ ] V046 — `V000046__local_close_durable_notices.sql`
- [ ] V047 — `V000047__local_privacy_retention_purgers.sql`
- [ ] V048 — `V000048__local_defense_synthetic_evidence.sql`
- [ ] V049 — `V000049__local_m04_gallery_cover.sql`
- [ ] V050 — `V000050__local_publication_retirement.sql`

### V051–V060
- [ ] V051 — `V000051__local_close_reservation_after_settlement.sql`
- [ ] V052 — `V000052__scoped_publication_rate_retirement.sql`
- [ ] V053 — `V000053__local_close_contract_and_publication_retention.sql`
- [ ] V054 — `V000054__local_contract_signature_expiry_retention.sql`
- [ ] V055 — `V000055__mpv2_02_public_search_and_locations.sql`
- [ ] V056 — `V000056__mpv2_04_public_gallery.sql`
- [ ] V057 — `V000057__mpv2_11_typed_public_category_filters.sql`
- [ ] V058 — `V000058__gadm_01_global_administrator.sql`
- [ ] V059 — `V000059__gadm_03_audit_scope_and_query_indexes.sql`
- [ ] V060 — `V000060__gadm_local_bootstrap_restricted_role.sql`

### V061–V070
- [ ] V061 — `V000061__mpv2_12_hourly_publication_schedules.sql`
- [ ] V062 — `V000062__gcp04_durable_privacy_replay_registry.sql`
- [ ] V063 — `V000063__gcp04_privacy_suppression_claim_fencing.sql`
- [ ] V064 — `V000064__gcp04_privacy_cleanup_worker_role.sql`
- [ ] V065 — `V000065__gcp06_bootstrap_audit_actor.sql`
- [ ] V066 — `V000066__gcp06_test_access_reset.sql`
- [ ] V067 — `V000067__inmuebles_residential_rentals.sql`
- [ ] V068 — `V000068__img_01b_media_schema.sql`
- [ ] V069 — `V000069__img_01c_local_media_provider.sql`
- [ ] V070 — `V000070__img_01c_durable_media_write_intents.sql`

### V071–V080
- [ ] V071 — `V000071__img_01c_terminal_media_write_intents.sql`
- [ ] V072 — `V000072__img_01d_media_relation_trigger_scope.sql`
- [ ] V073 — `V000073__img_01e_privacy_media_cleanup.sql`
- [ ] V074 — `V000074__first_business_action_roles.sql`
- [ ] V075 — `V000075__auth_evaluation_consent_documents.sql`
- [ ] V076 — `V000076__spaces_gallery_capacity_replacement_v2.sql`
- [ ] V077 — `V000077__img01b_gallery_slot_record_guard.sql`
- [ ] V078 — `V000078__img01b_gallery_replay_expected_revision.sql`
- [ ] V079 — `V000079__document_verification_cases.sql`
- [ ] V080 — `V000080__document_verification_space_ownership_supersession.sql`

### V081–V084
- [ ] V081 — `V000081__document_verification_runtime_scope_helpers.sql`
- [ ] V082 — `V000082__document_verification_retention_precedence.sql`
- [ ] V083 — `V000083__document_verification_lock_and_transfer_fix.sql`
- [ ] V084 — `V000084__didit_identity_verification_schema.sql` — conflicto de autoridad y privacidad identificado; falta cerrar ledger/DDL/contrato durable.

## 9. Encargo posterior: consolidación local y release correctivo V085

El usuario pidió el 2026-11-07 consolidar primero el estado local y después migrar el desarrollo Cloud SQL a V85 tomando local como referencia. Se interpreta como **fuente versionada y probada**, no como permiso para copiar datos sobre Cloud SQL, reescribir V1–V84, editar ledger o saltar pendientes/guards. No se inventa V85 vacía para cambiar un número; contendrá solo correcciones demostradas y compatibles. El alcance previo solo de análisis queda ampliado a implementación/pruebas y preparación del release, conservando backup, revisión y verificación exacta antes de aplicar cualquier cambio persistente.

Secuencia de trabajo actual:

1. Completar matrices individuales V001–V084 con revisión independiente, evidencia por hash/objeto/consumidor y distinción entre defecto confirmado e hipótesis. Auditorías paralelas acotadas: `auditoria_v001_v020_20261107.json`, `auditoria_v021_v040_20261107.json`, `auditoria_v041_v060_20261107.json`, `auditoria_v061_v084_20261107.json`. No considerar un archivo recibido como cierre automático.
2. Resolver conflictos locales con tests y delta append-only V085 si está justificado. Probar fresh, V76→V85 y V84→V85 con datos sintéticos; conservar V1–V84 byte a byte. La corrección tardía no puede saltar una precondición de una versión anterior que impida llegar a V85.
3. Alinear la base nativa únicamente tras respaldo/revisión y verificación de su identidad/autoridad. El snapshot renovado READ ONLY de las **16:20:31 -03** confirma V1–V76 y pendientes V77–V84; 76 nombres/checksums coinciden. Evidencia completa: `evidence_native_ledger_20261107_tarde.json`. Todavía no se aplicó DDL nativo.
4. Preparar y probar el modo privado `preflight-only` sin backup/apply/rollout, respetando el ejecutor canónico. Implementado en working tree en `release_main.py`/`cloudbuild.yaml`; flag CLI y sustitución `_DB_PREFLIGHT_ONLY` deben coincidir. El modo sí requiere lock, imágenes/provenance, artefactos y un Job de plan, así que no se presenta como una operación cloud sin efectos externos/costo. **No se ejecutó en GCP** ni se publicó el código.
5. Leer ledger/owners/ACL actuales de Cloud SQL por ese modo revisado. La metadata consultada confirmó proyecto `espacigo-testprod`, instancia `espacigo-testprod-db`, región `southamerica-west1`, motor `POSTGRES_18`, estado `RUNNABLE`; no prueba autenticación SQL ni versión del esquema. No se usó un túnel ni se consultaron datos de negocio.
6. Solo después del cierre local y del preflight remoto: backup válido → aplicación secuencial de pendientes hasta V085 por Job privado → readback de ledger/checksums/catálogo. No desplegar KYC ni atribuir persistencia funcional por llegar a V85.

Inventario canónico nuevo: `migraciones_inventory_v001_v084_20261107.json`, generado por el **CLI real** `go run ./cmd/dbmigrate -inventory-only -dir db/migrations`, sin conexión DB, y validado por `verify_migration_inventory.validate`. Fingerprint reproducible del runner: `3921ff747e4a7e7f49de443de7333dad03535f0687078b93b8a3cac6042a209a`. El TSV anterior se conserva como evidencia histórica; no se reutiliza su raíz de serialización no documentada como digest de release.

## 10. Resultado local V085 y prerrequisitos persistentes — actualización 2026-11-07

- [x] Candidato `V000085__identity_encrypted_declarations_and_minimal_events.sql` implementado; SHA-256 `7b78094558e2d9bde7804919b01c54d34caf2ac31ac34fb1b86a63999aeeaf51`. Declara snapshots cifrados, intentos ligados por cuenta/revisión y eventos mínimos sin cuerpo/URL/firma; reduce escritura legacy. No implementa endpoints ni persistencia KYC de aplicación.
- [x] PostgreSQL desechable real: **95 resultados requeridos PASS sin skips**, fresh V1→V85, upgrades V75/V76→V85, V83→V84 y V84→V85 restringidos, rollback/ledger, catálogo/API, permisos y negativos tipados. Filas legacy bloquean V85 sin borrado. Roundtrip cifrado usa keyring Go real y datos sintéticos. Entorno y límites en `contrato_v85_identidad_20261107.md`.
- [x] Go race/vet y `go test -count=1 ./cmd/dbmigrate` PASS. Python discovery completo `/usr/bin/python3`: **87 tests PASS**; ResourceWarnings de fixtures HTTP sintéticos conservados, no se instalaron dependencias ni se consultó Cloud desde esos tests. `git diff --check` PASS.
- [x] CLI real genera inventario V1–V85 en `migraciones_inventory_v001_v085_20261107.json`; fingerprint `fba8f6351e14622dbe8d65ae8c594d3ff73bf63a96976bbd7dd7a52417f8bc23`. V1–V84 coinciden byte a byte con HEAD. `git ls-remote` confirmó main remoto/local tracking en `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`, cuyo inventario termina en V84; no se creó/pushó commit.
- [x] V85 registrada **solo como revisión requerida** (`security-tightening`, `automatic=false`), no como expand automático. Su revocación de permisos legacy y guard de ausencia de datos requieren compatibilidad y revisión específicas; prueba Python confirma que el release automático continúa rechazándola. No retirar ese guard para satisfacer la palabra «forzar».
- [x] Native READ ONLY agregado: a las 16:47:37 -03 no se observaron empates de clave V14 ni snapshots nulos V15; no existen tablas V84/V85. Estos resultados no reescriben historial ni prueban Cloud SQL. Evidencia: `evidence_native_authority_20261107_tarde.json`.
- [ ] Cerrar la revisión semántica manual de las 84 filas: se validaron todos los nombres/hashes, pero la extracción automática de dependencias/consumers produjo keywords y coincidencias no contractuales. Se pidió corregir esas matrices con lectura manual y referencias SQL reales, sin contarlas como cierre global.
- [ ] Resolver **prerrequisito nativo anterior a V81**: lectura de las 16:41:23 -03 encontró `espacigo_runtime` LOGIN y ausencia de `espacigo_api_login`, `espacigo_migrator_login`, `espacigo_schema_owner`, helper. V81 exige runtime NOLOGIN y esas identidades; V85 no puede saltarla. Preparar y revisar transición exacta de roles/autenticación/configuración y backup, sin deshabilitar un login existente por inferencia ni resetear contraseñas/cuentas.
- [ ] Aplicar la cadena nativa solo tras ese prerequisito, respaldo validado y gates del delta. El último ledger nativo leído sigue V76; **no se aplicó DDL persistente**.
- [ ] Publicación acotada de DB/pipeline, preflight-only privado del Cloud SQL exacto, ledger/owners/ACL actuales y revisión de V85 de seguridad; después backup y aplicación secuencial autorizada por Job. No mezclar/publicar cambios KYC parciales, no ejecutar desde laptop ni declarar versión cloud por metadata.

**Estado de cierre:** esquema candidato V85 probado en entorno local aislado; base nativa persistente y Cloud SQL NO están unificados ni actualizados por este resultado. No hubo backup persistente, cambio de roles/cuentas, Job remoto, despliegue, commit, push, merge ni modificación de migraciones históricas.

## Revisión parental de entregas y transición sintética (2026-11-07)

- Se recibieron nueve informes pequeños V1–V84. El padre contrastó **84/84 nombres, versiones y hashes con archivos y bytes de HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`**: las migraciones históricas siguen intactas. Esa comprobación no cierra semántica ni destinos.
- La validación inicial de 217 referencias encontró 170 diferencias de indentación y cinco extractos parciales en vez de líneas completas. Además hay fallos semánticos que un hash/excerpt correcto no detecta: V62 apuntó a un claim de cola en lugar del registry, y V51–V60 presentó tests como consumidores de producción. Las entregas no se aceptan por sus autoevaluaciones.
- `correcciones_parentales_referencias_20261107.json` aporta **28 filas corregidas y 50 referencias completas verificadas** (consumidores, definiciones de tests y binding histórico). Distingue SQL real, CLI restringida, consumo indirecto por trigger y contraevidencia de estado en memoria. No afirma ejecución de esos tests ni que las otras filas estén aprobadas.
- `review_v031_v040_semantica.json` queda **rechazado como informe final**, conservado como evidencia de revisión; se preparan dos reemplazos de cinco versiones. V39 sí modifica columnas para desvinculación por privacidad y tiene consumidor en `identity/retention.go`; no está justificada su lista vacía anterior.
- **Hallazgo V77:** el predicado `position(original_fragment IN replace(function_definition, original_fragment, '')) > 0` no detectaba duplicados; no se observó un fallo de destino por ello. Se implementó un guard previo del runner dentro de la transacción de V77 y se probó el rechazo de duplicado sin alterar función/ledger en harness PostgreSQL aislado; véase la sección 7 (97 resultados obligatorios PASS, sin skips). No se editó V77 ni se pretende reparar su precondición con una V85 posterior; la búsqueda de este patrón en migraciones no encontró otro archivo.
- El harness anterior al guard dio **96 resultados requeridos PASS sin skips**; ese resultado histórico fue supersedido. Tras implementar el guard V77, `go test -run '^TestV77'` y el harness PostgreSQL completo registraron **97 resultados requeridos PASS, sin skips**, incluido el negativo `upgrade-v76/v77-duplicate-predecessor-blocks`. La evidencia es de fuente/fixture sintético; no valida el estado de nativo ni Cloud SQL. Detalle y límites en la sección 7.
- Continúan abiertos la revisión semántica final por fila, la transición/identidad/backup del nativo y la autoridad/backup/autorización de aplicación Cloud. El ledger Cloud ya fue leído: no queda pendiente la lectura inicial; sí permanece V084 pendiente y CREATE/REFERENCES ausentes para el actor observado. No hay DDL persistente, cambios de credenciales/cuentas, publicación o integración por estas auditorías.

### Matriz de trabajo única tras corregir las entregas

`matriz_semantica_consolidada_v001_v084_20261107.json` reúne **84 filas únicas ordenadas y 212 referencias calificadas de línea completa**, con 0 referencias no calificadas en el estado actual de la matriz. Usa los reemplazos V31–V35/V36–V40, una revisión parental nueva de los SQL completos V51–V60 (`review_v051_v060_parental.json`) y correcciones adicionales de consultas por columna, triggers y definiciones de pruebas. No se seleccionan por glob informes viejos y nuevos solapados.

El follow-up sustituyó las referencias fixture/no-test previas de V18, V19, V70 y V73 por declaraciones Test y aserciones con su alcance explícito. V67 tiene `v67-inmuebles-seed`, que fija categoría, los 34 códigos/órdenes y atributos requeridos, límites/unidades; sus baselines empty/V75/V76 pasaron en la corrida completa anterior. V19 añade `TestCurrentSchemaMigrationV19WeeklyHours`, que aplica únicamente V1–V19 y prueba en PostgreSQL los triggers originales de fixture/tarifa, modalidad/zona y las restricciones de rango/solape; su ejecución específica pasó 11 resultados requeridos. La matriz conserva cobertura PostgreSQL parcial para V68/V70/V73. El test V1–V61 sigue representando estado final tras V61, no sustituye el test de prefijo V19. V9 y V46 sí tienen tests con aserciones/cadena de llamada cualificadas. V84 tiene contraevidencia explícita del map en memoria y ningún consumidor PostgreSQL confirmado: es un hallazgo source-only, no se inventó uno para llenar la casilla. V39 ya enumera las tablas alteradas y sus UPDATE NULL de retención; V42 apunta a los bloqueos, no al historial de reseñas; V46 apunta al repositorio de avisos, no al render de texto.

La matriz conserva `semantic_audit_closed=false`: la revisión parental final de inventarios de objetos/dependencias/autoridad por fila y los límites de cobertura V68/V70/V73 siguen abiertos; no quedan referencias no calificadas ni grupos de test vacíos. La corrida completa anterior al test V19 registró 100 resultados PASS sin skips, incluido V67 y el guard V77; el scope V19 separado registró 11 resultados requeridos PASS. El harness completo actualizado no se volvió a ejecutar. Ambas evidencias son sintéticas, no de destinos persistentes. Unificar referencias y evidencias de fuente no unifica ledgers ni prueba mínimos privilegios efectivos de un destino.

### Guard V77 — evidencia de versión de ensayo, reconciliada con el harness actual

La entrega delegada del guard fue incompleta: sus subtests no llamaban al guard y por eso el PASS inicial no probaba ningún caso. El padre reemplazó esas pruebas por ejecuciones reales del helper y reprodujo RED: el fragmento Go mantenía comillas SQL duplicadas aunque `pg_get_functiondef` devuelve el cuerpo decodificado. Se corrigió únicamente la constante del runner, no V77. Las pruebas ahora comprueban una ocurrencia, cero, definición vacía, duplicado, fallo de catálogo y función ausente, preservando el error original, además de equivalencia con el literal escapado de V77.

El guard consulta la función dentro de la transacción de la migración pendiente V77 y antes de su SQL; no corre sobre V77 ya aplicada ni migraciones ajenas. El nuevo subtest obligatorio `upgrade-v76/v77-duplicate-predecessor-blocks` introduce un comentario inerte con segunda ocurrencia en el cuerpo de la función del fixture V76. Verifica rechazo explícito `found 2`, cero migraciones aplicadas, todas las filas del ledger sin cambios y definición de función intacta; restaura su propio fixture y la cadena siguiente llega a V85. No demuestra un incidente en nativo/Cloud SQL ni autoriza reparar sus funciones.

Validación real del árbol posterior al guard:

- `go test -count=1 -run '^TestV77' -v ./internal/migrator`: PASS, incluidos los seis casos y comprobación del literal histórico.
- `bash scripts/test-migration-runner-postgres.sh`: **97 resultados requeridos PASS, sin skips**, fresh V1–V85, upgrades V75/V76, releases restringidos y negativos. Contenedor propio `espacigo-migration-ci-1570266`, imagen PostGIS fijada, sin red/puertos/volúmenes, PGDATA tmpfs/socket privado; retirada y ausencia verificadas. Log local: `/home/nandev/.hermes/cache/scratch/espacigo-v85-v77-guard.log`.
- `go vet ./internal/migrator`, `go test -count=1 ./cmd/dbmigrate` y `git diff --check`: PASS.
- `/usr/bin/python3 -m unittest discover -s scripts/gcp -p 'test_*.py'`: **87 tests PASS**. Salieron ResourceWarning de fixtures HTTPError; los mensajes de build/release/schema de esta suite son respuestas simuladas, no ejecución remota ni lectura de Cloud SQL.
- Nueva comparación byte a byte: V1–V84 iguales a HEAD; SHA256 candidata V85 sigue `7b78094558e2d9bde7804919b01c54d34caf2ac31ac34fb1b86a63999aeeaf51`.

Ese resultado registra la corrida anterior de 97 obligatorios, antes de añadir el test V67. El follow-up inmediato que sigue la supersede como conteo vigente del harness, pero no como evidencia de destino persistente.

## V67 seed inmuebles — test PostgreSQL específico (2026-11-07 23:11 -03)

- Se añadió `v67-inmuebles-seed` a `TestCurrentSchemaMigrationFromEmptyAndV75` y el checker exige sus resultados para `empty`, `upgrade-v75` y `upgrade-v76`. Las aserciones consultan PostgreSQL real del harness: categoría/nombre/orden, perfil v1, los 34 códigos en orden, atributos requeridos, límites/unidad del área construida y duración mínima/máxima.
- `bash scripts/test-migration-runner-postgres.sh`: **100 resultados PostgreSQL obligatorios PASS, 0 skips**; incluye los tres subtests V67 y el negativo V77. Contenedor propio `espacigo-migration-ci-1911489`, imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`, `network=none`, sin puertos ni volúmenes, PGDATA tmpfs y socket privado. El script verificó la retirada del contenedor.
- `python3 -m unittest discover -s scripts/gcp -p 'test_check_migration_ci.py' -v`: **2 tests PASS**, incluido rechazo de omisiones/skips.
- `env -i PATH="$PATH" HOME="$HOME" GOTOOLCHAIN=local go test -run '^$' ./internal/migrator`: compilación de paquete/tests PASS, sin ejecutar otra vez los casos.
- Se actualizó la matriz y recibos: 212 referencias calificadas, 0 no calificadas, `test_versions=[]`; V67 sale de los gates pendientes. Sigue `semantic_audit_closed=false` por signoff parental, cobertura parcial V68/V70/V73, V84 sin consumidor durable y gates separados de nativo/Cloud SQL.
- Solo hubo DDL/fixtures dentro del PostgreSQL desechable autorizado, retirado por el script. Ninguna base persistente, credencial, permiso, ledger persistente, migración histórica, Job Cloud, despliegue, commit o push fue modificada.

## V19 guards de horario semanal — prueba PostgreSQL de prefijo (2026-11-07 23:32 -03)

- Se añadió `TestCurrentSchemaMigrationV19WeeklyHours`: copia las primeras 19 migraciones inmutables a un directorio temporal, las aplica solo en su base desechable y verifica activación con/sin fixture habilitada, última tarifa hora/no hora, rangos de día/duración, exclusión de solapes, cambio de zona/modalidad y tarifa no horaria.
- La primera ejecución aislada falló correctamente: el fixture insertado después de aplicar V1–V19 no tenía la fila de tarifa que V10 habría backfilleado para espacios preexistentes. Se corrigió el setup sintético para crear explícitamente la tarifa v1 antes de probar V19; no se modificó ninguna migración.
- `bash scripts/test-migration-runner-postgres.sh v19`: **11 resultados requeridos PASS, 0 skips**. Contenedor propio `espacigo-migration-ci-1934107`, imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`, `network=none`, sin puertos ni volúmenes, PGDATA tmpfs y socket privado; retirada verificada. Solo ejecutó el test específico V19.
- El runner admite scopes fijos `full`, `v19` y `v76`; el scope `v76` ejecuta solo TestGalleryCapacityTransitionV68ToV76 y exige siete resultados nombrados. El scope completo permanece separado, requiere 111 resultados y no incluye el test v76 dedicado; no se ejecutó en esta fase.
- `python3 -m unittest discover -s scripts/gcp -p 'test_check_migration_ci.py' -v`: **3 tests PASS**; `bash -n scripts/test-migration-runner-postgres.sh`: PASS; `go test -run '^$' ./internal/migrator`: compilación de paquete/tests PASS, sin ejecutar otras pruebas.
- Matriz/revisión/plan actualizados a 212 referencias calificadas, 270 anclas de línea exactas, 0 no calificadas y V19 ya no figura como brecha de test. `semantic_audit_closed=false`; V68/V70/V73, signoff final, V84 sin consumidor durable y los gates separados de nativo/Cloud siguen abiertos.
- Revalidación 2026-11-08 01:05 -03: matriz con 84 filas, 212 referencias, 270 anclas exactas y hashes V001–V084 intactos; JSON parentales parseables y `git diff --check` limpio. `go vet ./internal/migrator` PASS tras los cambios de código. El scope completo actualizado (111 resultados) sigue sin ejecutarse.
- La corrida inicial fallida y la corrida PASS usaron únicamente sus contenedores desechables; ambos fueron retirados. No hubo consultas/escrituras a PostgreSQL nativo ni Cloud SQL, ni cambios de ACL, ledger persistente, despliegue, commit o push.

## V68→V76 capacidad de galería — revisión source-traced (2026-11-08)

- **Evolución intencional, no migración pendiente:** V68 (`db/migrations/V000068__img_01b_media_schema.sql:120,150,232`) define el estado inicial de 10: `position BETWEEN 1 AND 10`, rechaza backfill legacy sobre 10 filas activas y el trigger rechaza un nuevo slot cuando `occupied >= 10`. V76 (`db/migrations/V000076__spaces_gallery_capacity_replacement_v2.sql:4,139`) reemplaza el CHECK por 1–30 y el cupo transaccional por 30. No editar V68 ni agregarle 30: eso rompería la historia inmutable.
- **Compatibilidad de la versión del producto:** en el padre del commit V68 `5995a5fcdbd428220cd8927499d1d33f84466980` el dominio Go todavía fijaba `MaxPhotosPerSpace=10`. El commit `c50a2ef22dd2e3fb3134d0cb8bfe248fcedb7f6d` añade V76 y cambia ese contrato a 30 en el mismo cambio. Por lo tanto la discrepancia observada es supersesión explícita, no un desajuste de esas dos versiones fuente; la secuencia de release sigue requiriendo migración antes de desplegar la aplicación dependiente.
- **V77/V78:** V77 (`V000077__img01b_gallery_slot_record_guard.sql:6–19`) solo sustituye el acceso del campo exclusivo de `media_asset` por lectura segura del record y mantiene el cupo. V78 (`V000078__img01b_gallery_replay_expected_revision.sql:4–29`) añade el par target/expected_revision e inmutabilidad; no cambia el cupo. Las búsquedas de `gallery`, `media_asset`, `space_media` y `espacio_galeria` en V79–V84 no devolvieron cambios a esa semántica.
- **Consumidores actuales:** Go fija `media.MaxSpaceMedia=30` (`internal/media/domain.go:15`) y `gallery.MaxPhotosPerSpace=30` (`internal/gallery/domain.go:12`); el handler devuelve `max_photos` desde esa constante y mensajes de límite con 30 (`internal/gallery/transport/http/handler.go:313,316,533,563`). El adaptador reconoce ambos mensajes históricos/actuales (`internal/adapters/postgres/media/repository.go:940`), con prueba de mapeo para 10 y 30 (`internal/adapters/postgres/media/repository_error_test.go:18–25`). La UI Next fija `MAX_PHOTOS=30` (`frontend/src/features/host-listings/ui/listing-gallery.tsx:26`); las pruebas visibles esperan 30 (`frontend/tests/phase7-host-listings.spec.ts:2141–2142,2720–2722`; `frontend/tests/phase-space-gallery-native.spec.ts:381`). Los tests unitarios de constantes son `internal/media/space_limit_test.go:5–8` e `internal/gallery/domain_test.go:5–8`.
- **Pruebas/evidencia:** `TestSpacePhotoReplacementAtomic` sigue siendo una prueba opt-in de IMG01F para el flujo completo de reemplazo V76/V78; requiere `127.0.0.1:15432/espacigo_f6_synth_20261028` y storage configurado y no se ejecutó ni se usó ese destino. Para el hueco específico 10→30 se agregó `TestGalleryCapacityTransitionV68ToV76` y se ejecutó `bash scripts/test-migration-runner-postgres.sh v76` en PostgreSQL desechable, con 7/7 resultados obligatorios PASS y 0 skips: backfill de diez filas, rechazo de slot 11 bajo V68, preservación de filas/grant al aplicar V69–V76, ledger final 76, competencia por cupo 30, aceptación de la posición 30 y rechazo del 31.º. La evidencia está en `planning/gcp/evidence_gallery_capacity_v68_v76_20261108.md`. No aplicó V77–V84 ni prueba la integración IMG01F, concurrencia al límite histórico 10, backfill legacy de más de diez, ACL efectiva o estado de destinos persistentes. El harness `scripts/test-img01b-migrations-postgres.sh` es obsoleto y no se ejecutó: omite V68 y crea el hueco V68 entre V67/V69 que `DiscoverMigrations` rechaza; si se superara, su expectativa `max(version)=68` no coincide con el inventario actual.
- **Decisión:** no añadir una prueba V68-only que consagre 10 como contrato vigente. La transición específica 10→30 ya quedó cubierta por el scope V1→V76 autorizado; la cobertura sigue parcial solo para backfill con más de diez filas bajo V68, concurrencia en el límite histórico 10, flujo completo IMG01F/V78 y ACL efectiva. V68 no se modifica. No se aplicó DDL ni se consultaron destinos persistentes; `semantic_audit_closed=false`.
- **Revalidación 2026-11-08 02:01:23 -03:** matriz V1–V84 con 84 filas, 217 referencias calificadas, 280 anclas exactas, 0 no calificadas ni discrepancias; los 84 SHA-256 coinciden con matriz y manifiesto. JSON de matriz/reviews/final/parentales parseables y referencias nuevas al test V68→V76 exactas. `scripts/test-migration-runner-postgres.sh v76`: 7/7 PASS, 0 skips, limpieza del contenedor verificada; checker Python 4 tests PASS, `go vet ./internal/migrator` y `git diff --check` PASS. No se ejecutó el scope full (111 resultados), ni se accedió a DB persistente/Cloud SQL. HEAD y rama sin cambios; worktree sigue sucio con trabajo KYC y otros cambios preexistentes preservados.
