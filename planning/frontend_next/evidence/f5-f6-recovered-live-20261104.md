# F5-A3 y F6 — reejecución local tras recuperación (2026-11-04)

## Estado verificado

Backend: `codex/backend/feature/local-close-02-espacios-galeria-documentos`, código probado/publicado `8d33dcfce3992719d11f5067c0b55ebf6e032e32` (base funcional `d7fb5c45759ff527e9187bb5aebda431a9033e92`; se conservó el commit documental posterior del usuario). Frontend: `codex/frontend/fix/local-close-02-f6-native-e2e`, HEAD final `cc2fdc39670477b5c051ea74c853bb905604fb4d`, base `a50f62ce00fd8558abf5e1d321692d59e34ad863`. Ambos refs de código publicados con hooks e identidad HernanEspinozaDev y confirmados por `git ls-remote`/API de commit. Este documento y la lista reconciliada se cierran en un commit documental posterior de backend, sin cambiar las fuentes probadas. El recibo final de entrega registra además ese HEAD documental remoto exacto. No integración main, PR ni cloud.

Target exacto: `127.0.0.1:15434/espacigo_f5_gate_20261101`, server `172.28.0.2`, PG18.6. V1–V83 y checksums verificados tras restauración. Recuperación ACL y sus pruebas separadas: [evidencia de baseline](f5-restored-acl-baseline-20261104.md).

## Identidades y respaldo

Autorización humana explícita: solo dos cuentas de reemplazo mediante registro/verificación normales y Mailpit del ensayo. Ambas registro201/verificación204, login/sesión/logout confirmados por el ejecutor. Readback independiente del padre en el target exacto: dos UUID del inventario presentes y activos; no se imprimieron valores.

Inventario privado nuevo `.local/testing/f5-gate-20261101-f5a3/f5-a3-accounts-20261104T074455Z.json`, archivo0600/carpeta0700. Se conservó el inventario original. El padre corrigió un error de metadata del ejecutor: había usado puerto API18082 en `target.port`; tras comprobar las cuentas en la DB15434, separó `api_target` y fijó `target.port=15434`. No modificó credenciales ni identidades.

Backup posterior a cuentas y fuera de tmpfs: `/home/nandev/.local/share/espacigo-f5-backups/f5-post-replacement-accounts-20261104T074549Z.dump`, 647860 bytes, SHA-256 `97a8d72f8e496ad62e945d5080d62bee315440d1463d76431b21dd14d5f98b06`; TOC/schema parse PASS. No detener PostgreSQL para limpiar recursos temporales: su data es tmpfs.

## Suite PostgreSQL documental completa

La API worker-free propia se detuvo antes del paquete. Target/credenciales separados, IP esperada y scanner `172.25.0.2:3310`; OWNER_ID/REVIEWER_ID cargados en memoria desde el inventario, nunca desde argumentos impresos.

1. `TestPostgresCommittedReviewAndRevoke`: PASS (0.122 s de paquete), con denegaciones añadidas y limpieza/readback.
2. `go test -json -mod=readonly -timeout=540s ./internal/adapters/postgres/documentverification -count=1`: exit0, **86 tests top-level PASS, cero FAIL y cero SKIP**. No se usó `-skip` ni `-run` para restringir el paquete.
3. Log JSON privado `.local/testing/f5-gate-20261101-f5a3/document-suite-20261104.jsonl`; conteos extraídos programáticamente de eventos terminales top-level, no sumando subtests.
4. Los tres gates committed están explícitamente presentes y PASS: review/revoke, retención y scanner real (antes excluido). No atribuir esta ejecución a un stub o a rollback-preview.

## Matriz F5-A3

| Criterio | Evidencia ejecutada | Límite |
|---|---|---|
| Carga | `TestPostgresUploadPersistsEncryptedObjectAndPendingScanJob` PASS | Integración Go/PostgreSQL/storage local; no browser |
| Inspección fail-closed | Suite inspección real local y pruebas de upload/scanner PASS | Poppler/parser local, sin documentos reales |
| Scan real durable | `TestPostgresCommittedRealScannerOutcomesAreVisibleAfterCommit` PASS con ClamAV1.5.4/firmas28173, clean/EICAR | Scanner sintético local; sin cloud |
| Revisión/revocación | `TestPostgresCommittedReviewAndRevoke` PASS, commits separados y readbacks independientes/auditoría | Inspección del fixture marcada sintética; no implica acceso binario para reviewer |
| Denegaciones | Owner sin grant review/revoke y capacidad space sobre caso identity; estado/revisión/eventos intactos | Casos negativos añadidos no sustituyen todo posible scope/grant revocado |
| Descarga | `TestPostgresSpaceDownloadHoldsOwnerLockThroughStorageRead` PASS, serialización y denegación después de transferencia | Prueba con transacción exterior revertida; distinguir de los tests committed |
| Retención durable | `TestPostgresCommittedRetentionDeletionIsVisibleAfterCommit` PASS usando cuenta existente, sin crear/borrar usuarios | Ambigüedad de respuesta COMMIT no inyectada en esta prueba |
| Permisos | Readback ACL completo y `TestPostgresV83ReviewerGrantLockPrivilegeIsMinimal` PASS | Baseline restaurado revisado, sin grants nuevos improvisados |
| Concurrencia | Claimers retención, fence scan/transferencia, lock descarga, carreras V81 y precedencia V82 PASS | Casos de carreras controladas del paquete, no prueba de carga productiva |
| Cleanup | Postflight independiente: casos0/archivos0/jobs0/grants activos0/clientes ajenos0; eventos append-only preservados | Los grants revocados y eventos históricos no se borran |

El paquete completo y sus readbacks quedan aprobados como evidencia local de sus criterios. En ese punto F6 visible seguía pendiente; el resultado posterior está en la sección final. No equiparar el cierre técnico local a aceptación humana del producto o del plan transversal completo. Hardening adicional de commit ambiguo y más combinaciones de permisos se conserva como brecha de cobertura específica, no como fallo ocurrido en esta ejecución.

## F6 — secuencia diagnóstica histórica de esta reejecución

API propia worker-free reiniciada solo para el borrador real del spec (`proc_fe31b81d0310`, PID2600563), loopback18082. No se contactaron procesos protegidos18083/18084. Se adapta únicamente la selección del inventario privado del spec para usar las dos cuentas nuevas conservando el default histórico y controles de ruta. Luego ejecutar borrador en worker-free y flujo documental en scan-only, sin mocks ni browser del usuario.

Primera ejecución real Playwright con inventario nuevo: fixture de selección de ruta PASS, borrador worker-free FAIL por timeout120s. Snapshot saneado muestra wizard sin opciones de categoría. Diagnóstico fuente y API: categorías sin Authorization→401, con sesión→200/nueve items; perfil oficina autenticado→200. `OpenApiHostListingsAdapter.listCategories` y `getCategoryProfile` omitían Authorization; no es falta de catálogo ni bloqueo PostgreSQL. Corrección en curso pasando la credencial existente por hook/fachada/puerto/adaptador y ambos consumidores, sin global token, cookies ni aperturas públicas. La UI y permisos aceptados no se rediseñan.

El spec recibió espera acotada a la opción oficina para localizar la frontera en15s en vez de esperar120s, y una descripción sintética válida para el mínimo de100caracteres del esquema ya inspeccionado. El artefacto standalone debe reconstruirse después de cambios productivos y antes del browser GREEN; un build viejo no valida fuente nueva.

Tras corregir catálogo y reconstruir standalone, el browser llegó al paso2: el ID histórico `attr-has_wifi` no existe en el perfil actual; se cambió el selector a checkbox accesible `Wi-Fi` observado en la UI/schema, no se inventó un atributo. El siguiente POST real fue rechazado por BFF400 `header_not_allowed`: createDraft enviaba Idempotency-Key aunque Go/OpenAPI POST/spaces no lo soportan. Se retiró únicamente ese header del adaptador de creación (galería mantiene su contrato), con regresión ejecutable3/3 PASS, typecheck/lint/build PASS. El spec exige ahora ausencia del header no soportado, no idempotencia ficticia.

El siguiente rechazo vino de Go422 con detalles saneados `region_code/comuna_code:invalid_value`; se corrigió el fixture a región13/comuna13101 según `validPublicLocationCodes`, que exige códigos numéricos2/5dígitos y prefijo común. La etiqueta actual que admite nombre de comuna no demuestra soporte de nombres; es un límite de UX detectado, no permiso para relajar Go.

La ejecución posterior llegó a Go422 `attributes:unknown_field`. La causa fuente es que el wizard agrega `cancellation_policy` y opcionalmente `security_deposit_clp` a los atributos sin pertenecer al perfil. Corrección acotada en curso para enviar solo atributos del perfil/version real y no ofrecer depósito como capacidad operativa inexistente. No ampliar schemas/ACL ni ignorar errores para pasar el recorrido.

Revisión independiente `deleg_04dfe5bc` aprobó la propagación Bearer del catálogo y la retirada del header no soportado en creación; confirmó galería intacta. Repitió3/3 tests locales y señaló correctamente que las dos pruebas de categorías son guardas estáticas de fuente, mientras la de createDraft ejecuta el adaptador con transporte sintético. El browser real ya demostró carga del catálogo/perfil en el target; no confundir esas dos fuerzas de evidencia.

Cambio preexistente ajeno detectado en backend `.gitignore`: exclusión `/base de datos/` con comentario de documentación local solicitada. Se preserva sin incluirlo automáticamente en los commits del corte ni revertirlo; condiciona la afirmación final de worktree limpio si no se coordina su propiedad.

## F6 — resultado técnico local final

La corrección del wizard pasó RED/GREEN ejecutable: solo atributos dinámicos y versión exacta del perfil cargado; sin perfil se bloquea creación. Cancelación permanece en normas; depósito no soportado deshabilitado con explicación accesible. Build standalone regenerado y probado en el padre. No se modificaron contratos Go/OpenAPI, allowlist BFF ni idempotencia real de galería para forzar el flujo.

1. Perfil realmente `local-http-worker-free`, Next4203→BFF→Go18082: `@fixture|@local-http-worker-free`, **2 PASS/0 FAIL/0 SKIP** sobre espacio NUEVO con sufijo sintético `20261104-final1`. Creación201, lectura persistida, reanudación/editor y logout204.
2. API propia cambiada explícitamente a `local-document-scan-only`, mismo target/espacio/cuentas; `@f6`, **4 PASS/0 FAIL/0 SKIP**. Incluye selección segura del inventario, fixture PDF válido, preflight y recorrido visible crear caso→upload202→inspección→ClamAV real→limpio→submit200→readback independiente→descarga UI PDF A4 de una página→otro owner404→logout204. No mocks, no browser del usuario ni lectura online.
3. Se corrigieron dos defectos del spec, no del backend: enviar devuelve `enviada` (no `en_revision`) y cancelar la descarga antes de pedir su body a Chromium podía dar cero bytes. Ahora se exige `enviada` exactamente tras submit y se lee `download.createReadStream()` del archivo realmente descargado, con firma PDF y `pdfinfo`. Las restricciones loopback y same-origin se comprueban por separado.
4. Frontend global: API inventory185 vigente, lint/types/formato/build PASS, tooling **80 PASS/0 FAIL/SKIP**. Playwright general **69 PASS/48 SKIP/0 FAIL** sin opt-ins; esas omisiones no son integración aprobada y los opt-ins del corte están acreditados arriba. Se corrigió la expectativa mock histórica que exigía el header no soportado; después del último cambio test-only de accesibilidad/captura, wizard mock focalizado1/1PASS y lint/types/formato PASS.
5. Captura recortada sintética del depósito deshabilitado inspeccionada por el padre: sin identidad, email, UUID ni credenciales. Versionada en la rama frontend junto a `planning/frontend_next/evidence/f6-native-close-20261104.md`. No se afirma aceptación visual humana.

## Postcheck y conservación de evidencia

Readback independiente READ ONLY al final: DB/IP/usuario/PG18.6 exactos; ledger83, min1/max83; **2 casos enviada, 2 archivos limpio, 2 jobs completado y cero grants activos**. Son los dos recorridos browser (diagnóstico y corrida fresca), retenidos como evidencia sintética: NO confundirlos con los ceros del postcleanup de la suite PostgreSQL previo al browser. No se borraron casos enviados ni auditoría append-only para maquillar limpieza.

Backup final fuera de tmpfs: `/home/nandev/.local/share/espacigo-f5-backups/f5-f6-post-browser-20261104T083403Z.dump`, 653008 bytes, modo0600, SHA-256 `8205068b4426eb6483e8ed54b7f61f6bb12b6d4e8a3e44702741c1cdcc9952ca`; TOC y parse schema-only PASS. Storage cifrado y clave privada permanecen bajo `.local/testing/` excluido, no en Git. PostgreSQL y firmas tmpfs no deben detenerse para retirar un forward/API.

`go test -mod=readonly -timeout=300s ./... -count=1` y `go vet -mod=readonly ./...` repetidos PASS en el padre con entorno limpio sin DSN; checker schema12/12PASS. No atribuirles la suite live86/86: esta permanece acreditada separadamente arriba, con las mismas fuentes funcionales.

El usuario autorizó explícitamente incluir `/base de datos/` en un commit separado de `.gitignore`, sin versionar contenido del directorio; no revertir cambios ajenos. El HEAD backend avanzó a `4e26d52b57d9687c4b6b7a778e89dbbb5be855e5` por un commit documental del objetivo ratificado, conservado y sin reescribir historia. No cambia las fuentes funcionales de la suite live.

Gates técnicos F5-A3/F6 satisfechos dentro del alcance sintético local descrito. Revisión independiente final `deleg_0df92f8a` PASS, sin hallazgos de seguridad o lógica. Sugirió robustecer la extracción de fuente del test de payload; no es fallo productivo ni gate omitido. Se suman las revisiones aprobadas de retención/review-revoke, SQL de preparación y baseline referidas en el plan y la evidencia específica. La ejecución runtime corresponde al padre, no a los revisores source-only.

Retirada efectuada y verificada: API propia PID2661348 detenida; forward propio PID2529223 retirado; sockets15434/18082/4203 sin listeners. La primera guarda de retiro del forward falló antes de tocarlo porque `/usr/bin/socat` resuelve al ejecutable `socat1`; se inspeccionó el proceso exacto, se verificó el comando loopback15434→172.28.0.2:5432 y se retiró sin relajar el destino. Red temporal `espacigo-f5-clamav-update-egress-20261029` eliminada después de confirmar cero adjuntos. No updater ni egress de actualización retenidos.

Se conservaron healthy el PostgreSQL tmpfs `espacigo-f5-gate-db-20261101`, scanner `espacigo-f5-clamav-synth-20261029-recovered-20261104` con firmas RO y Mailpit existente `espacigo-f6-synthetic-mailpit-20261029`. Motivo/responsable: evidencia sintética de este corte pendiente de revisión humana; evitar otra pérdida de DB/firmas y preservar correo retenido ajeno a la limpieza. No se autoriza borrar/detener esos recursos por inferencia. Inventarios, passfiles y clave cifrada conservados privados, ya que la evidencia/demo aún los requiere; no copiados a Git ni a otro destino.

Publicación directa sin force-push ni reescritura de historia. `.gitignore` autorizado quedó separado en `3985234f0442c66c47eee217b996171c6b9431b2`, incluido en la rama backend. No se consultó CI remoto ni se le atribuye PASS.

No hay autorización para main/F7/cloud. Siguen siendo límites explícitos, no capacidades implementadas: revisión humana de documentos reales/acceso binario de reviewer, depósitos, idempotencia POST/spaces, UX de nombre de comuna y variantes de fallo/permiso no ensayadas. El objetivo transversal ratificado y el resto del plan no se cierran por inferencia de este corte.
