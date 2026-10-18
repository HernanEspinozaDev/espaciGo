# Cierre técnico del ensayo GCP-03 g03o-1017 y propuesta GCP-04

Fecha de evidencia: 2026-10-18. Estado del ensayo: verificación post-restore completada. Este cierre resume un ensayo técnico run-scoped; no declara GCP-03 completo, GCP-06 final, aceptación de producto ni operación productiva.

## Resultado observado

PR #408 se fusionó en main con merge e74e000c5c46b95639b3eb8f680c5f719d751299 (HEAD 9f413793a36438de3269101fee038e76b9fd1282). La corrección incluye las referencias de archivos privados vivos que faltaban en el conjunto canónico.

| Fase | Evidencia observada | Límite |
| --- | --- | --- |
| continuation-plan | Ejecución tpnvl: plan_ready, phase_success=true, report_complete=true, retry=false, execution_performed=false. 3.245 referencias DB, 3.246 archivos y 808.343 bytes; plan SHA-256 dcde67c5f6413d31ae3b60a038728613c0414889a59c758c2429e4f713d21b0b. | Solo comprobó precondiciones y produjo el plan de copia; no escribió archivos. |
| Copia create-only | Ejecución 7jnv2: status=continued, stage=complete, phase_success=true, report_complete=true, transaction_outcome=committed, retry=false. Misma imagen aprobada; una tarea; timeout 1800s. | Una copia del bucket privado de este ensayo, no la migración final GCP-06. |
| Inventario GCS posterior | La lectura independiente de metadatos contó 3.246 objetos, 808.343 bytes y metadata SHA-256 en cada objeto. El SHA canónico sha256(nombre):tamaño:metadata.sha256 es b5df1ac2c9fbf93da19f5abb170ad7287303799768e5c5e75f1e42e7249bad06. Coincide con el reporte de dominio y el inventario sellado. No se descargaron payloads durante esta lectura. | No compara el contenido de cada objeto mediante una segunda descarga; comprueba conteo, tamaños y hashes SHA-256 declarados en metadata. |
| Verificación post-restore | Ejecución xw28v: status=verified, tarea completa, database_rows_emitted=false. Las diez comprobaciones fueron verdaderas: database, postgres_version, extensions, migration_history_matches, application_tables_match_toc, application_object_owners, observer_role_active, database_role_is_not_superuser, runtime_role_disabled y sanitized. 96 tablas públicas; inventario de migraciones SHA-256 eca6b4295b593bc3ea3af470fb747bd6e788e141df4a57c8a0fae91ea6ad1fb2; inventario de tablas SHA-256 5d78e5d55d9021a935b7210977395c3dae2540c983e4266107b2583619aaabe1. | Verificación de criterios definidos del ensayo; no es comparación exhaustiva de todas las filas o DDL del origen. No acredita compatibilidad de la instancia Cloud SQL que se seleccione para runtime; el destino de este ensayo fue run-scoped. |

## Cambios de state e infraestructura del ensayo

La imagen OCI aprobada se publicó una vez y Artifact Registry devolvió el digest sha256:9c6856a44e27806f8f0f9c2773092735b6e2b88152f96b87774be1dcd697d9ca; el archivo fuente OCI tenía SHA-256 60b409433a2a2bf72f2d73f30c128d05b001bd8dd20ecb53031510b2855e41e4.

El state GCP-03 conservó lineage 4e81a953-7353-fbd4-93a8-e4b9a5019e95 y avanzó de serial 22 a 25 por tres planes guardados revisados y aplicados:

1. Serial 22 → 23: actualización de imagen del Job a continuation-plan; 0 altas, 1 cambio, 0 bajas.
2. Serial 23 → 24: actualización del Job a continuation-copy y binding al SHA del plan de dominio; 0 altas, 1 cambio, 0 bajas.
3. Serial 24 → 25: alta del Job verifier, identidad run-scoped, usuario SQL observer y permisos IAM mínimos condicionados; 6 altas, 0 cambios, 0 bajas.

Los Jobs y recursos run-scoped permanecen preservados para revisión. El plazo de preservación se extendió explícitamente hasta 2026-10-18T12:00Z; ese plazo no es una autorización de borrado automático. No se tocaron los SQL baseline, el checkout principal, stashes ni recuperación ajena.

## Historial de fallos y corrección

Los intentos anteriores siguen formando parte del historial y no deben confundirse con el resultado final:

- La ejecución inicial de importación se detuvo en OWNER_ACL_NORMALIZATION_FAILED con transaction_outcome=uncertain.
- Una ejecución de recuperación posterior detuvo su validación de referencias privadas en PRIVATE_FILE_REFERENCE_MISMATCH después de registrar transaction_outcome=committed.
- Después se amplió la consulta de referencias vivas, se obtuvo plan_ready, la copia se completó y el verificador post-restore confirmó sus diez comprobaciones.

El diagnóstico Cloud anterior al saneamiento informó pending_notices=958; la recuperación posterior confirmó commits de normalización/saneamiento y postverify informó sanitized=true. Por separado, el restore-copy local anterior dejó 958 avisos cuarentenados en su fixture local. Son contextos y conjuntos distintos; no trasladar el conteo local al estado Cloud ni omitir que el Cloud tuvo 958 avisos pendientes antes del saneamiento.

## Evidencia privada

La evidencia completa permanece fuera de Git en .local/testing/g03o-1017-private-reference-set-35dff4a-20261018/terraform/ con permisos 0600. Los archivos agregados para revisión son:

- continuation-plan-evidence-tpnvl.json — SHA-256 09d61f3daf734d9be138cb42e63bd667c7afbeecfee96fda20e2a48774c3b57d.
- continuation-copy-evidence-7jnv2.json — SHA-256 91aba3f527c8ce5d501acc7a71c76a934242e3a5f34b489696753e13186ddc37.
- continuation-copy-gcs-inventory-evidence.json — SHA-256 86fc4797bfd08df36bf530f09145e36514bee5ad84e4551850e7670a80ff290d.
- Verificación independiente del revisor reviewer-gcs-copy-verification.json — SHA-256 a35715f6e854108976ac954627c68102189a3cdc48301abcade805cb05e7650f.
- postrestore-verification-evidence-xw28v.json — SHA-256 d4402b52d158714fd8c91d3a8339c7f97b1eef2fcef40edb5bcc454511ef1bcd.
- Attestation independiente del revisor reviewer-postrestore-verification.json — SHA-256 6425c392d0fe80bb8a3b62d9367938c3db7e70d4f9607bf21d4d2c35c6cdb112.

No se versionan state, plan binario, tfvars, logs, nombres de objetos privados, filas ni payloads.

## Estado vigente de GCP-03

El ensayo g03o-1017 concluyó técnicamente las etapas de copia y post-restore definidas para este run. GCP-03 sigue parcial hasta revisar la compatibilidad de la instancia Cloud SQL que se elija para GCP-04 (IAM DB, extensiones/roles, permisos, red y recuperación), el alcance de datos de la migración final, rollback/costos y aceptación humana. g03o-1017 usó un destino run-scoped; las instancias baseline no fueron necesariamente su destino y no se exige comparar exhaustivamente todas por defecto. GCP-06 sigue pendiente: la copia privada run-scoped de 3.246 objetos no equivale a una copia final coherente de DB, archivos, claves y configuración. GCP-05 (Angular/SSR/gateway/dominio) y GCP-07 (E2E, rollback, costos y entrega) también siguen pendientes.

## Propuesta concreta del siguiente corte: GCP-04 backend durable

El siguiente corte propuesto prepara y valida un backend durable aislado. Esta sección es un plan de trabajo, no una capacidad ya desplegada ni autorización de apply. El protocolo para el smoke real con GCS/Cloud SQL, sus fases, costo y retención se desarrolla en la [propuesta específica de GCP-04](gcp04_cloud_replay_smoke_proposal_20261018.md).

### Bloqueos verificables que el corte debe resolver

- Replay compartido: implementado localmente en PR #411, después de la fusión de PR #410 (merge `14d958bc12308ad0a86c5f6f2453c8e0062c02a8`). Se conserva V62 byte a byte: el ledger transaccional sigue siendo local a PostgreSQL y un restore antiguo puede retrocederlo. El backend optativo `LOCAL_PRIVACY_REPLAY_BACKEND=gcs` guarda un manifiesto mínimo agregado en un objeto privado único y usa la generación observada de Cloud Storage como compare-and-swap. `filesystem` sigue siendo el default. El adapter trata conflicto 412 mediante recarga y merge; una misma ejecución con contenido incompatible detiene la operación. Respuestas perdidas se reconcilian leyendo de nuevo; si no se puede confirmar durabilidad, el API responde 503 después de que el borrado de cuenta ya quedó confirmado en DB y el worker vuelve a intentar en su siguiente ciclo. La recuperación de un backup viejo es explícita: ejecutar `local-privacy-replay-apply` con el manifiesto respaldado antes de iniciar API/workers; el comando combina la entrada con el registro GCS vigente y elimina blobs usando el mismo `BLOB_STORE_BACKEND`, bucket y prefijo configurados para runtime. Este comando ahora crea el pool con el mismo constructor que el runtime, por lo que también respeta `DATABASE_CONNECTION_MODE=cloudsql` y la selección de IP del Cloud SQL Go Connector; no se comprobó contra Cloud SQL real. Un fallo de eliminación remota mantiene el replay pendiente y no marca la evidencia como removida. La entrada del comando admite hasta 16 MiB para cubrir el máximo del manifiesto y rechaza entradas mayores. No hay recuperación automática al arrancar.
- Alcance y límites observados del replay: pruebas locales de servicio y de protocolo con `httptest` cubren CAS, colisión, reintento y respuesta perdida; no son integración real con GCS. No se configuraron bucket, IAM, Terraform, secretos ni Cloud Run, y no se ejecutó deploy. El objeto tiene tope de 16 MiB y el dominio conserva el máximo de 10.000 entradas; es adecuado para el volumen bajo esperado, no demuestra escalabilidad indefinida. La política de retención del objeto debe alinearse con la ventana de backups antes de habilitarlo en Cloud. Si no se configura GCS, la ruta de sidecar local sigue igual y V62 sigue siendo el único registro durable en DB.
- Workers y leases: varios reconciliadores y workers se inician como goroutines del proceso API. Inventario de código: pagos, garantías y liquidaciones cada 5 s; caducidad/aprobación de contratos cada 5 s; avisos de credenciales cada 1 s; avisos locales cada 1 s; limpieza de supresiones cada 10 s; limpieza de operaciones, fotos y galería cada 15 s; retención local de privacidad y contratos/documentos locales cada hora. Los avisos locales reclaman fila con lease de 30 s y lock advisory por destinatario; los avisos de credenciales reclaman fila con lease de un minuto y completan/reintentan solo con el lease esperado. La retención de verificaciones y las limpiezas de operaciones/galería usan claims con `SKIP LOCKED` o funciones SQL y marcas temporales de recuperación; estas marcas no son tokens de fencing. La limpieza de supresiones lee pendientes sin lease, aunque su finalización vuelve a bloquear la fila y es idempotente. Los sweeps de reservas/garantías/contratos dependen de locks de filas y transiciones de estado, no de un lease de líder. Por tanto dos procesos pueden repetir llamadas externas; la revisión no demuestra coordinación global ni fencing de efectos externos.
- Migraciones: `cmd/api/migrate.go` restringe la ruta local a `LOCAL_AUTH_PROTOTYPE`; el runtime de API no debe ejecutar DDL. El runner activo `cmd/dbmigrate` usa el historial canónico V1–V62, con checksums y lock de migración. Exigir ese historial para el Job separado y usar rol/identidad separados. Las herramientas históricas GCP-03 (`cmd/gcp03scratch`, `Dockerfile.dbmigrate` y su selector de inventario OCI) están intencionalmente fijadas a V1–V61; no son el migrador runtime y no se modifican en este corte.
- Pool y red: DATABASE_POOL_MAX_CONNS acepta 1–50 y por defecto vale 10, pero el límite agregado depende de máximo de instancias, tareas, workers y otros clientes. Definir presupuesto de conexiones contra el tamaño real de Cloud SQL, concurrencia, timeouts y política de salida privada.
- Secretos e IAM: fijar versiones numéricas, identidades separadas para API/migrador/worker, acceso mínimo y sin valores en logs. Verificar conectividad privada, roles y recuperación de Secret Manager sin habilitar servicios durante este corte.
- Pruebas: ejecutar el replay integrado local con PostgreSQL 18 desechable, endpoint HTTP falso de GCS y dos procesos OS reales: CAS concurrente/412, respuesta de escritura perdida, export tras reinicio, restauración de snapshot pre-supresión y reaplicación del manifiesto, y Delete fallido que conserva evidencia pendiente. Esto acredita el contrato local de replay y un reinicio del proceso; no valida GCS/IAM reales, Cloud Run multi-instancia, lease takeover general, límites agregados de conexiones, ni migrador único en Cloud.

### Entregables y gates

1. PR de código/contratos para almacenamiento de replay, workers, leases, migrador, pool y configuración de secretos; pruebas locales enfocadas y reporte de límites.
2. Preflight Cloud de solo lectura de organización, inventario de red/Cloud SQL, roles, cuotas y facturación; cotización vigente para cada recurso propuesto.
3. Plan Terraform exacto con recursos, región, costo, conexiones agregadas, rollback y teardown selectivo; revisión independiente del plan y sus archivos privados.
4. Aplicar infraestructura mínima y ejecutar smoke sintético solo después de revisar el plan exacto en este chat y recibir la liberación operativa para esa fase bajo la autorización ya delegada por el usuario. No pedir una segunda confirmación genérica para una operación dentro del alcance; una acción nueva o destructiva fuera de él requiere autorización específica. Separar API pública/Angular y datos persistentes a GCP-05/06.
5. Aceptar GCP-04 solo cuando el restart test y dos instancias validen replay/leases, el migrador sea único y checksummed, el pool permanezca bajo presupuesto y las identidades/secretos sean comprobados.

### Retención y retiro del ensayo

Mantener los recursos run-scoped, objetos privados y evidencias hasta terminar la revisión del cierre y decidir retención. La propuesta es preparar después un inventario exacto y un plan Terraform de teardown solo para recursos etiquetados g03o-1017, con segunda lectura del state y de dependencias. No eliminar los objetos GCS ni los artefactos locales como parte de ese plan; cualquier retiro de contenido requiere retención acordada y autorización independiente. No tocar instancias baseline, Artifact Registry compartido, state, snapshots, llaves en custodia, checkout principal, stashes o recuperación ajena.

### Actualización de implementación local — PR #411 y siguiente corte

PR #411 se fusionó en `main` por `HernanEspinozaDev`, merge `3014d3be35927e0e1eed908b8471fdbffc7aad4c`. El siguiente corte agrega la validación integrada descrita arriba y corrige el comando local de replay para usar el constructor de pool del runtime. La prueba se ejecuta en PostgreSQL 18/PostGIS desechable; no agrega migraciones ni cambia V1–V62. El defecto observado y corregido fue que `FailSuppressionFile` infería `$3` como intervalo al sumar el backoff, aunque la columna es `timestamptz`; el error SQLSTATE 42804 impedía registrar el reintento, y el servicio ocultaba ese error. El parámetro ahora se convierte explícitamente a `timestamptz` y los errores al registrar el fallo se propagan. El resultado operativo sigue siendo preparación local, sin despliegue ni aceptación humana. El endpoint HTTP falso valida el contrato ejercitado por el cliente, no IAM, retención, latencia ni comportamiento de un bucket real. No hubo acceso Cloud, publicación de imagen ni prueba con Cloud SQL real.

### Actualización del ensayo GCP-04 `g04replay18` — 2026-10-18 22:05:06 UTC

El smoke GCP-04 `g04replay18` fue retirado selectivamente bajo la revisión independiente registrada en la matriz [`gcp04_g04replay18_status_matrix_20261018.md`](gcp04_g04replay18_status_matrix_20261018.md). El state quedó vacío (serial 9, SHA-256 `78ef07d3fde85defd63dd90dc1fd6fdfcae3b521c25edbdd8048b0ba64e47a86`) después de once bajas run-scoped; la evidencia privada de cierre tiene SHA-256 `15e7d2ae1cbd255e81fdba0254d547cd96827ece41aee541130151843884cd5b`. Se confirmó que los recursos exactos ya no están disponibles y que las instancias GCP03origin, testprod y pilot permanecen RUNNABLE. El código de salida numérico no se retuvo, así que no se afirma ni se obtuvo repitiendo el teardown. Los objetos GCS soft-deleted conservan el plazo original; no se cambió su retención ni se solicitó purga.

El corte GCP-04-W1 implementa un worker local separado para la cola de borrado de evidencia de privacidad. V63 agrega claim SQL atómico con lease, owner y generación de fencing; completion y retry requieren el token vigente. La API deja el trabajo en cola y conserva el export de replay; Docker/Compose incorporan solo el worker local. Cloud Storage usa `ifGenerationMatch` sobre la generación leída: protege contra reemplazos durante la operación, pero no da fencing remoto ni exactamente-una-vez.

El código se publica en [PR #417](https://github.com/HernanEspinozaDev/espaciGo/pull/417), abierto hacia `main` por `HernanMEC`, con revisión solicitada a `HernanEspinozaDev`.

Verificación: `go test ./...` pasó; con PostgreSQL 18/PostGIS efímero pasaron `internal/adapters/postgres/identity` y `internal/migrator` en secuencia, incluida base nueva V1–V63, actualización V62→V63 y pruebas de dos procesos de worker/takeover. El replay integrado con endpoint GCS falso también pasó. Compose config y el build Docker del worker pasaron. No se desplegó Cloud, no se probó IAM/Cloud SQL/Cloud Run y GCP-04 sigue pendiente de sus demás gates. La matriz de estado registra tanto el retiro del ensayo como el alcance y límites de W1.
