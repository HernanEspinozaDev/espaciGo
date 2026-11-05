# F5 — Reconciliación, delta de persistencia e integración requerida

**Corte:** 2026-10-29. **Estado:** diseño/propuesta revisada; no constituye migración aprobada ni integración productiva. No se aplicó DDL ni se conectó a PostgreSQL en este corte.

**Checkout:** `/home/nandev/Documentos/projects/espaciGo`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`, HEAD base `c50a2ef22dd2e3fb3134d0cb8bfe248fcedb7f6d`; cambios F5 locales no commiteados.

## Reconciliación de notas anteriores

- El snapshot del plan de **2026-10-27** (“F5 bloqueada/sin contrato”) es histórico. La política de owner del 2026-10-28 y las aclaraciones explícitas del 2026-10-29 prevalecen.
- La nota del **2026-10-28** que dejaba sin resolver si el administrador actual podía revisar/revocar queda supersedida: ese administrador no puede revisar ni revocar el caso de su espacio aunque el sujeto histórico sea otra persona; tras transferencia se exige caso nuevo.
- La nota inicial del **2026-10-28** sobre ausencia de ClamAV también es histórica. La prueba autorizada del 2026-10-29 contra el scanner local actual está en `espacios-f5-clamav-prueba-local-20261029.md`: el adaptador real clasificó clean/EICAR; la suite local cubre errores fail-closed.
- Las decisiones sobre asignación por persona no autorizan roles heredados: `review_identity` y `review_space` son capacidades distintas, con asignación explícita. El código local actual exige que `ReviewerID` coincida con el actor, capacidad única y scope exacto; **la semántica persistente del scope y el roster/cola siguen sin aprobación**.
- La nota antigua decía que el target F6 había sido retirado. Eso describe un corte anterior, no el actual: la autorización F6 específica y `espacios-f6-autorizacion-auth-local.md` registran el target sintético activo; la inspección actual confirmó que el contenedor sigue `running`. No se usó ni modificó para F5.

## Inventario vigente y evidencia

- El esquema base verificable está en `db/migrations/V000001__m01_identity.sql` (`usuario`), `V000003__m03_verification.sql` (`verificacion`, referencia `fixture:` y proveedor fijo `local-fixture-v1`, KYC/KYB) y `V000005__m04_space_drafts.sql` (`espacio.propietario_id`). Ninguna de esas tablas representa un expediente documental F5.
- La última migración es `V000078__img01b_gallery_replay_expected_revision.sql`; no existe `V000079`. `internal/migrator/migrator.go:75-103` exige secuencia contigua y verifica nombre/checksum históricos, por lo que V79 es solo candidato, nunca se reservó ni escribió.
- `V000068__img_01b_media_schema.sql` crea `media_asset`/`space_media` con scope propietario/espacio, pero sus variantes están restringidas a `image/webp` y dimensiones; sirve de referencia de constraints, no como storage documental.
- El runtime actual crea media filesystem bajo `M03_EVIDENCE_DIR/private-media` (`cmd/api/runtime_storage.go:20-35`); `internal/adapters/mediafs/store.go:18-46` obliga raíz privada y solo acepta WebP. No se debe reaprovechar para documentos ni arrancar con raíz inferida.
- Contratos locales de descarga (`internal/documentverification/download.go:10-20`) exigen una lectura autoritativa que devuelva el caso, owner actual y actor, y store por IDs de caso/archivo, no por object key del cliente. El handler está definido en `internal/documentverification/transport/http/owner_download.go:15-36`, pero la búsqueda de `cmd/api` no halló registro de `RegisterOwnerDownloadRoute`; tampoco existe adapter F5 de Postgres/store ni construcción runtime/worker del scanner ClamAV.

## Delta mínimo de persistencia propuesto — pendiente de aprobación técnica

La propuesta es aditiva, sin migrar ni copiar fixtures antiguos. Las tablas y constraints exactos requieren revisión Go/SQL y prueba en un target F5 aislado antes de ejecutar DDL.

| Objeto propuesto | Datos/garantías requeridos |
|---|---|
| `verificacion_documental_caso` | `id`, propósito identidad/espacio, sujeto/cuenta histórica, `space_id` nullable solo para identidad, estado, revisión CAS, vencimiento del documento, timestamps de decisión/vigencia/revocación. FKs a `usuario`/`espacio`; constraints propósito↔space; índice único parcial de un caso abierto por sujeto de identidad y por espacio. Cada operación de espacio vuelve a validar `espacio.propietario_id`; nunca hereda un caso al transferirse. |
| `verificacion_documental_archivo` | ID opaco, caso, bytes/tamaño/MIME/páginas/hash, estado técnico (`pendiente`, `limpio`, `bloqueado`, `error`), fecha y versión/frescura de firmas, código de error sanitizado, referencia privada no elegible por el cliente y plazo/estado de retención. No almacenar bytes, nombres originales, PII ni contenido del scanner en DB. |
| Asignación de revisores | Grant asignado explícitamente a `ReviewerID`, una capacidad y scope. El esquema exacto de `review_identity`→sujeto y `review_space`→espacio sigue siendo propuesta del corte local, no decisión productiva: requiere confirmar qué scope se autoriza, cómo se asigna/revoca el grant y cómo se excluye una persona que reciba ambas capacidades. Hasta entonces, mantener denegación fail-closed; ningún rol general o soporte hereda binarios. |
| `verificacion_documental_evento` | Historial append-only: actor, caso/archivo lógico, acción/resultado, código de motivo, revisión, correlación y timestamp. Sin bytes, clave de objeto, nombre, token o detalle libre sensible. Grants DB limitados para impedir update/delete ordinario. |
| Outbox/job de inspección y retiro | Job idempotente con tipo, archivo/caso, estado, lease, intentos, disponibilidad y último código sanitizado. Permite persistir `pendiente` antes de enviar a ClamAV y reconciliar la eliminación del store; el contenido nunca se declara `limpio` por timeout/error. El worker se añade solo con runtime de retención autorizado y probado. |

**Orden transaccional requerido:** lock de cuenta activa → lock del espacio cuando corresponda → caso y archivos; verificar actor autenticado, owner actual, revisión CAS y grants desde autoridad persistida; ejecutar la transición y registrar historial/outbox en la misma transacción. Para aprobar antecedentes de espacio, verificar identidad documental vigente del administrador actual dentro del límite transaccional. Un CAS de fila aislado no evita carrera con transferencia, revocación o vencimiento.

## Integración requerida, en orden

1. **Cerrar el contrato de asignación** (scope exacto, grant dual, roster/cola y quién puede administrar asignaciones). La regla de conflicto de interés/transferencia ya está decidida; no reabrirla.
2. Preparar y revisar una migración incremental candidata **V79** con FK/check/index y privilegios mínimos `espacigo_runtime`; no reutilizar `verificacion` fixture ni tablas/roles de F6. Cero escritura hasta identificar un target F5 aislado autorizado.
3. Implementar adapter PostgreSQL para `CreateCase`, `Transition` y lectura de descarga desde una única vista/tx autoritativa; integrarlo con `SpaceDownloadRepository`. Agregar test real que verifique CAS stale/no partial writes, aislamiento entre sujetos/espacios, grants por reviewer ID/scope, bloqueo del admin actual y transferencia.
4. Implementar filesystem privado F5 en raíz distinta de media/evidence, con permisos restrictivos, claves derivadas solo de IDs server-side, cuarentena/no-follow/path-safe, tamaño/hash comprobados y reconciliación de objetos fuera de transacción. No arrancar la API con una raíz implícita.
5. Añadir carga multipart/cuota y outbox: guardar objeto en cuarentena, inspeccionar ClamAV local, y en una transacción marcar limpio solo ante respuesta `OK` vigente; error, timeout, EICAR, respuesta parcial o scanner no disponible dejan el archivo no descargable y registran el resultado mínimo.
6. Registrar el handler de descarga existente en `cmd/api` solo después de inyectar adapter/store autorizados; añadir autenticación y OpenAPI. Añadir APIs de creación/carga/estado/revisión/revocación con `If-Match`/revisión; no expandir soporte.
7. Conectar identidad documental persistida al gate de booking (ambas partes) y al distintivo de espacio/revocación/vencimiento. Actualmente el gate de booking es fail-closed/preflight, no transacción durable de verificación.
8. Añadir BFF/UI y workers de expiración/retención/retiro. Tests HTTP/handler con fakes siguen siendo útiles, pero no reemplazan el test integrado browser→BFF→Go→PostgreSQL→store privado ni la evidencia de retención/versiones.

## Puertas antes de integración real

- No reutilizar `espacigo_dms_stage_20261020` ni `espacigo_f6_synth_20261028` para F5; no iniciar F6, no tocar cloud, no DDL ni escrituras en esta continuación.
- La política autoriza desarrollo y prueba con datos sintéticos, pero el requisito documentado es un target aislado identificado y aprobado; actualmente no hay target PostgreSQL F5 autorizado. Para pruebas reales, primero acordar el target exacto y pasar preflight, backup/restore verificable, revisión independiente de V79 y adapter, luego ejecutar integración con DB exclusivamente sintética.
- La prueba de handler de descarga ejercita autorización antes del store con fakes, no una ACL/DB/storage productiva. El ClamAV real solo prueba scanner-boundary clean/EICAR; las fallas de red/timeout/frescura se reproducen con stubs locales. No declarar persistencia ni workflow completo.

## Retención y cleanup exacto de ClamAV

**Motivo de retención:** se conserva la instancia scanner autorizada para continuar pruebas sintéticas del borde scanner→persistencia cuando exista un target F5 aprobado; evita repetir el pull/update de firmas y conserva la evidencia reproducible. El contenedor `espacigo-f5-clamav-synth-20261029` está `running/healthy`, digest oficial fijado, IP `172.25.0.2:3310`, solo en red interna sin egress ni puertos publicados. El ancla `espacigo-f5-clamav-synth-20261029-update` está `running/unhealthy` porque ejecuta `sleep infinity`, **sin redes** y sin actualización activa; su única función actual es mantener montado el volumen tmpfs de firmas. La red de egreso dedicada no tiene contenedores. Los volúmenes exactos son tmpfs (`...-db`, máximo 1 GiB; `...-quarantine`, máximo 256 MiB). No se está ejecutando FreshClam.

**Cleanup pendiente (no ejecutado porque F5 scanner→persistencia no está cerrado):** preservar antes la evidencia sanitizada; volver a inspeccionar por nombre exacto; detener y retirar únicamente los dos contenedores F5 anteriores; retirar solo los volúmenes `espacigo-f5-clamav-synth-20261029-db` y `espacigo-f5-clamav-synth-20261029-quarantine`; retirar solo las redes `espacigo-f5-clamav-synth-20261029-net` y `espacigo-f5-clamav-update-egress-20261029`; eliminar solo el scratch F5 cuando la evidencia esté resguardada; verificar la ausencia exacta de cada recurso. No usar `docker compose down --volumes`, `docker volume prune` ni limpieza global, y no tocar el contenedor/recursos F6. Ejecutar ese cleanup al terminar las pruebas F5 autorizadas o antes si el owner lo solicita.
