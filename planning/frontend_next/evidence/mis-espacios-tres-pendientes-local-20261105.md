# Tres pendientes de Mis espacios — evidencia local del 2026-11-05

Estado técnico: los tres recorridos afectados pasaron; continuación final del 2026-11-05 documentada al final, con revisión de los hallazgos y repetición del nativo sobre fuentes definitivas. Los SHA de código y entrega se registran en la matriz única y el recibo de publicación. Este documento no declara aceptación humana ni integración a main.

## Entorno y preservación

- Backend: `codex/backend/feature/local-close-02-espacios-galeria-documentos`, base publicada `4289139af7289aed0e152484ebedc02acb64b170`. Frontend: `codex/frontend/fix/local-close-02-f6-native-e2e`, base publicada `ce6d186b4f6c3c373ab69ab62690a0b93e5cd16f`. Las bases no son el SHA de los cambios nuevos aún sin commit.
- Único destino de aplicación: `127.0.0.1:15434`, `espacigo_f5_gate_20261101`. PostgreSQL y almacenamiento retenidos. API limitada, worker-free, en18082; Next standalone en4204 y TLS/BFF en4203. No se alternaron destinos, contraseñas o cuentas.
- Dos cuentas sintéticas existentes; política e inventario permanecen exclusivamente en `.local/testing/`, con permisos privados. No se registraron contraseñas, tokens ni identificadores de personas en esta evidencia.
- Se conservaron V1–V83, ACL del destino, evidencias de creación/galería/publicación/republicación y backups. No se ejecutaron reservas, firmas, cobros, garantías, main, F7 ni cloud.

## ACL operacional

`python scripts/run_local_semantic_acl_trial.py --authorized-local-synthetic-only` terminó en0 después del decoder estricto y de las pruebas de forma JSON.

- Referencia schema-only independiente, imagen exacta fijada, sin red ni puertos; referencia comprimida versionada con SHA del contenido fijado.
- Esquema equivalente con OIDs distintos: PASS. Privilegio extra, privilegio faltante y propietario incorrecto: rechazados en PostgreSQL, con rollback y comprobación de recuperación de cada negativo. DDL únicamente en la referencia.
- Gate completo del destino real: PASS; ledger83, bytes/checksums, grants directos/efectivos, propietarios y memberships. No se sustituyó por un preview terminado en rollback ni se agregaron grants para aprobarlo.
- El fingerprint numérico antiguo queda como diagnóstico histórico, no como segundo guard operacional. Decoder cerrado ante campos desconocidos, forma incompleta/null y JSON adicional.
- Log local: `three-gates-acl-strict-real-final.log`.

## Revisión documental real

Se prepararon tres PDF sintéticos nuevos mediante Go con la identidad runtime limitada: inspección real, ClamAV real, envío y eventos/leases de scanner. No se reutilizaron como prueba las filas antiguas sin archivos. Descarga oficial de firmas completada previamente, archivo de respaldo verificado y egress retirado.

Dos ejecuciones nativas distintas terminaron en **1PASS cada una**:

- Identidad: login de las dos cuentas existentes, denegación al titular, apertura/descarga desde UI, stream real SHA y `pdfinfo`, retiro confirmado de asignación, denegación de lectura y escritura, reasignación explícita, revisión intacta tras denegaciones y aprobación desde UI con GET autoritativo posterior.
- Espacio: mismo recorrido más denegación de un segundo expediente **del mismo titular y misma capacidad pero diferente espacio**, y revocación desde UI de la aprobación, con readback posterior.
- AXE completo desktop/mobile, sin suprimir las reglas de landmarks. Se corrigió el `main` anidado del componente y la secuencia de encabezados sin cambiar composición/clases visuales.
- Tres E2E controlados: PASS; decisión/revocación con readback, limpieza de formularios al cambiar expediente, purga de acciones tras403 y descarga con MIME correcto. Las regresiones de fachada ejecutan respuestas tardías y locks, no regex como sustituto de comportamiento.
- Readback SQL independiente: identidad `aprobada`, espacio revisado `revocada`, segundo espacio `enviada`; tres archivos limpios de620bytes con SHA esperado, scanner/firmas/inspección registrados, jobs `scan` completados con intentos y eventos `scanner/scan_result/limpio`. Permanecen dos cuentas y83 migraciones. El contador de intentos comprobado es el del job, no el campo legado homónimo del archivo.
- Logs locales: `three-gates-reviewer-native-identity-final.log`, `three-gates-reviewer-native-space-final.log`, `three-gates-reviewer-controlled-final.log`.

La prueba PostgreSQL de locks/callback y retiro con rollback acredita serialización; no sustituye la retirada **persistida** probada después por los recorridos nativos. Asignaciones del ensayo se preparan administrativamente de forma explícita y auditada; no se presenta un administrador de asignaciones como capacidad de producto.

## Calidad y recuperación de galería

Se conservan BEFORE84mediciones y AFTER12variantes en las evidencias del corpus autorizado. El piso humano es1280/Q75, solo cuando1600/Q80 excede614400bytes, siempre desde los píxeles originales. Si todavía excede el peso, se rechaza: no existe una degradación adicional ni presupuesto aumentado.

`TestDisplayBudget*` pasó y exportó entradas sintéticas opt-in para el navegador, con SHA y salida esperada después de comprobar contra un encode independiente del piso autorizado. JPEG y WebP se enviaron por el selector real, BFF y Go reconstruidos. El navegador descargó/decodificó ambos displays y thumbnails; comprobó SHA, MIME, dimensiones y bytes. Displays1280×960:611162 y558568bytes. Los thumbnails permanecen dentro de102400bytes. Un PNG válido que supera el presupuesto al piso fue rechazado sin cambiar las fotos ya guardadas.

Se detectaron y corrigieron dos problemas del recorrido de rechazo:

1. El BFF elegía `gallery/{photoId}` antes de la ruta literal `gallery/upload-status`; devolvía400 `header_not_allowed` pese al contrato correcto. Ahora la forma estática más específica se elige antes del método/cabeceras; no se ensancharon allowlists. Test del handler real RED→GREEN, incluido405/Allow. Consulta HTTPS/BFF independiente: antes400, después200 con estado `failed`.
2. La cola retenía cualquier fallo como pendiente y bloqueaba las demás acciones. Se añadió **Descartar carga fallida**, únicamente tras lookup autoritativo `failed`, sin replay ni cancelación remota. No se permite para `pending`, `not_found`, `deleting` o consultas/envíos en curso. Tres regresiones del motor y prueba nativa del botón protegen el criterio.

La corrida nativa final `20261105-floor5` terminó en **1PASS**: creación/edición/rehidratación, dos fallbacks exactos, rechazo al piso y descarte explícito, cola2/alfa96, portada/orden/reemplazo inválido y válido/borrado,30/31, AXE con galería poblada desktop/mobile, publicación persistida y denegación a otra cuenta. No modificó la galería histórica aceptada ni reprocesó sus archivos.

Los intentos `floor1`–`floor4` se conservan como pruebas parciales fallidas, no como aprobaciones: primero correlación incorrecta por posición de uploads concurrentes, después bloqueo de cola y lookup BFF. La comparación ahora se liga al contenido/SHA, no al orden de finalización. Log final: `three-gates-gallery-native-approved-floor-final.log`.

## Gates reunidos y demo retenida

- `go test ./...` y `go vet ./...`: PASS.
- Frontend: **138unitPASS,0SKIP**; TypeScript, ESLint y build de producción: PASS. Suite E2E global del artifact reunido: **75PASS/52SKIP**. Los skips opt-in no cuentan como integración; las tres ejecuciones nativas anteriores se registran separadamente.
- Demo documental: se retiró la asignación del expediente ya revocado y se asignó explícitamente solo el segundo espacio limpio pendiente. Retiro/alta y auditoría leídos tras commit. Queda una demo que puede abrirse sin resetear revisiones ni crear cuentas.
- Captura/ensayo nativo de demo **sin mutaciones documentales**,1PASS: login, abrir PDF real, AXE completo y sin overflow en1365×900/390×844, lectura final todavía `enviada`, logout. Capturas revisadas visualmente: `reviewer-pending-desktop-20261105.png` y `reviewer-pending-mobile-20261105.png` en la evidencia de la rama frontend; no muestran credenciales, UUID ni datos personales.
- Ruta de revisión: `https://127.0.0.1:4203/mi-cuenta/revision-documental`. Cuentas/materiales únicamente en el inventario privado ya existente. La galería nueva se identifica por el título sintético `20261105-floor5`; las demos y evidencias anteriores se preservan.
- Backup posterior al ensayo integrado fuera de tmpfs: comando de respaldo terminó en0 y verificó sus artefactos; servicios/forward/volúmenes retenidos para revisión humana, responsable agente padre/usuario. No se llamó limpieza a esta retención.
- El estado de publicación se rige por el recibo final y la matriz única; los SHA base mencionados arriba son historia, no prueba de entrega nueva.

## Continuación final: hallazgos y fuentes definitivas

### Lista y confirmación incierta

La revisión detectó que el catch postwrite purgaba selección, pero conservaba filas accionables. Se corrigió: cualquier escritura/confirmación fallida o revisión no avanzada borra **lista, selección y Blob URLs** y activa una barrera hasta un `load` explícito y exitoso. `select`, descargas y nuevas decisiones no pueden operar con las filas viejas. Una actualización que falla no levanta la barrera ni borra la advertencia de incertidumbre; una actualización exitosa recupera datos autoritativos y limpia el aviso. También se corrigió el mismo patrón ante un fallo de listado ordinario.

Regresiones definitivas: **16/16** de fachada. Browser controlado: **4/4**, incluido un test que recorre POST404 tras commit, GET500 y revisión obsoleta; comprueba los botones de ambas filas ausentes, selección/controles ausentes, un único POST, recuperación explícita y revisión2 aprobada. Un selector de alerta demasiado amplio incluyó el anunciador de Next; se acotó al texto de incertidumbre, sin suprimir el alert de producto. Se retiró una aserción regex del nombre de una variable porque la confirmación ya está cubierta con comportamiento real.

### 404 tras decisión guardada: reproducción PostgreSQL y HTTP

La hipótesis de que aprobar sustituye el **propio** expediente no se confirmó: la transición real no marca `superseded_at`. La prueba de servicio/repositorio con savepoint pasó y su rollback se declara como tal.

El escenario real reproducible es otro: `Review` confirma su transacción; antes de `Load` se retira legítimamente el grant; la lectura reautoriza y devuelve404. El ensayo opt-in `TestLocalReviewerCommittedApprovalThenWithdrawalHidesCase` ejecutó el handler HTTP real con servicio/repositorio reales y runtime limitado. Un wrapper controla solamente el instante de retirada: **después del commit de Review y antes de Load**. El retiro administrativo exacto confirma su auditoría; una conexión independiente READ ONLY verifica aprobación persistida, revisión incrementada, evento, grant revocado y caso no sustituido. Resultado real: **PASS**, HTTP404 y una única decisión. No se reseteó el expediente ni se disfrazó rollback de commit.

No se cambió el contrato para devolver metadata que ya no está autorizada. La UI trata cualquier respuesta postwrite no confirmada como incierta y no reenvía automáticamente. Una objeción de revisión que exigía retirar el grant después del commit de **Load** no corresponde al escenario original: ese orden no demuestra el404 por reautorización que se investigaba. Se conserva como observación rechazada por cambio de criterio, no como bug corregido.

### Procedencia y revisiones conservadas

`white_office` se reconcilió con Commons API, pageid61681790, artistaBreather yCC0. Padre verificó de nuevo el SHA-1 oficial contra las dos copias locales; ambas tienen2146421bytes y SHA-256 fijado. Los derivados BEFORE/AFTER son idénticos; no es una medición nueva. El recibo `gallery-white-office-provenance-20261105.{json,md}` sustituye explícitamente el caveat histórico403 del BEFORE inmutable. Los blockers del manifiesto quedaron separados como historia; se eliminó una afirmación de hash del rawAPI que no podía reproducirse a partir del JSON parseado.

Se conservaron las aprobaciones de ACL, codec y BFF/cola: los siete SHA-256 de sus archivos revisados se comprobaron sin cambios. La nueva revisión de fachada confirmó16/16 y sin bloqueos. La publicación incorpora la revisión de las dos pruebas PostgreSQL y de la reconciliación documental; los informes source-only no se presentan como ejecuciones de base/navegador.

Revisiones finales recibidas: `deleg_67ed9314` fachadaPASS/sinbloqueos y16/16 ejecutadas; `deleg_58769e65` procedenciaPASS/sinbloqueos y criterio404postwritePASS/sinbloqueos. La segunda revisión descartó expresamente la objeción de scheduling anterior por no corresponder al escenario. Aprobaciones conservadas: `deleg_0a9dafee` decoderACL, `deleg_0fd82878` codec (su caveat documental ya reconciliado) y `deleg_7f8caab6` BFF/cola. El padre verificó las ejecuciones PostgreSQL/nativas y los digests de fuentes, no solo los resúmenes de hojas.

### Gates definitivos, demo y límites

- Backend definitivo: `go test ./...` y `go vet ./...`PASS. Gate ACL real repetidoPASS; prueba real de locks/withdrawalPASS; normal-savepointPASS; commit real+retirada+HTTP404+readback independientePASS. Pruebas opt-in omitidas en el `go test` genérico se acreditan por sus ejecuciones explícitas, no por el skip.
- Frontend definitivo: **141unitPASS/0SKIP**, tipos/ESLint/API-check/formato/buildPASS. Browser global **76PASS/52SKIP**, con integración nativa acreditada aparte.
- Nativo documental repetido tras los cambios de confirmación: **1PASS** en5.0s, PDF real, denegación del titular, mismo titular/capacidad fuera de espacio, retiro persistido, reasignación sin mutación denegada, aprobación/revocación UI, GET posterior y AXE desktop/mobile. Log `finalreview-native-document.log`.
- Demo read-only final **1PASS** en3.3s: el expediente pendiente asignado conserva `enviada`, abre el PDF real y pasa AXE/overflow en ambos viewports. Capturas saneadas actualizadas en la rama frontend. Readback independiente: caso del ensayo404aprobado, espacio del nativorevocado, demoenviada, doscuentas,83migraciones y ungrant vigente.
- Backup final verificado fuera de tmpfs: recibo privado `backup-accepted-*` y log `finalreview-final-backup.log`, con digest de dump y storage y lectura completa de ambos archivos. API18082/Next4204/TLS4203/forward15434 y contenedores se **retienen**, responsable agente padre/usuario, para revisión; no se detuvo el último consumidor de un volumen tmpfs.
- **Scanner, límite explícito:** durante el ensayo la firma daily28173 cruzó las48h. Una nueva inspección quedó correctamente bloqueada con `scanner_unavailable`; no se marcó limpia ni se reiniciaron jobs a mano. Se repitió la descarga oficial acotada ya autorizada, pero `freshclam` informó daily28173 como la última disponible; egress desconectado y red temporal eliminada de nuevo. No se alteró el reloj, el piso48h ni ACLs. Ese borrador sintético bloqueado y su job se conservan separados como evidencia negativa. La demo de revisión abre archivos previamente inspeccionados y escaneados realmente limpios; no se promete que nuevas inspecciones pasen mientras las firmas oficiales superen la política. No afecta la aprobación/revocación/denegación nativa acreditada ni autoriza relajar scanner.
- No se crearon cuentas, no se aplicaron migraciones, no se alteraron ACL del destino ni se tocó main/cloud/F7/reservas/firma/cobros/garantía. Los dos SHA publicados se verifican mediante readback del ref remoto en el recibo final; aceptación humana de producto sigue separada.
