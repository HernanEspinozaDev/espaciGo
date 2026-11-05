# Mis espacios — desocupación, republicación y referencia ACL

Corte local sintético del 2026-11-04 (-03). Implementa los criterios de Mis espacios del ADR de esa fecha; no cierra los cortes de reserva, emisión/firma, cobros o garantía ni autoriza F7, main o despliegue.

## Código probado y revisión

- Backend: `b2d7c117b06b806f5c7a33725f85d0f7b1d47be8`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`.
- Frontend: `5dc7095d44770fc0e59b76b8e3dd1f1b515de3e5`, rama `codex/frontend/fix/local-close-02-f6-native-e2e`.
- Los commits documentales posteriores solo añaden esta evidencia, matriz y capturas. Los HEAD de publicación quedan en el recibo privado y en la entrega, sin autorreferencia dentro de un commit.
- Revisión independiente backend: PASS de fuente y paquetes Go, sin afirmar ejecución DB por el revisor. Revisión frontend detectó confirmación local reutilizable entre sesiones; corregida mediante key por sesión/permisos/espacio. Re-revisión PASS. No equivalen a aceptación humana.

## Matriz de criterios ratificados

| Criterio | Existía | Cambio de este corte | Prueba y resultado |
|---|---|---|---|
| Nueve categorías y perfiles dinámicos, incluyendo inmuebles | Catálogo/perfiles V67 y renderer; evidencia previa de nueve perfiles y 116 atributos | Sin taxonomía nueva ni formularios hardcodeados | E2E nativo lee las nueve categorías y cada perfil vigente; verifica Casa, Departamento, Pieza o habitación y Cabaña en opciones reales: PASS |
| Titular publica sin identidad contractual | Publicación separada de identidad; no añadir verificación contractual al anuncio | Conservado; guard de devolución no consulta identidad contractual | Readback SQL del titular: cero aprobaciones identity vigentes. Republicar desde UI devuelve 200 y reload mantiene activa: PASS |
| Solo titular vigente confirma desocupación después de entrega | Check-out finalizaba; recepción autorizaba por anfitrión histórico y no liberaba ocupación | Reutiliza reception; locks cuenta→espacio→reserva, revalidación de propietario actual y bloqueo administrativo antes de replay | PostgreSQL con rol runtime: antiguo anfitrión distinto del titular denegado, titular permitido, intervalo distinto rechazado. Nativo: otra cuenta 404, antes de checkout 409, checkout existente 201, recepción UI 201: PASS |
| Liberar únicamente el intervalo terminado | Ocupación tipada reserva, intervalo [inicio, termino), vínculo ocupacion_id | Desactivar solo ID/espacio/reserva/tipo/intervalo exactos en la misma transacción de recepción; fail-closed ante incoherencia/ocupación adicional | Integración real: contrato y hash de otras ocupaciones sin cambios, intervalo exacto inactivo y replay mismo resultado; rollback íntegro: PASS. Nativo confirma commit persistido por readback separado |
| Conservar contrato/historial y otros bloqueos | Tablas contractuales e historial independientes | Sin deletes contractuales ni barrido de ocupaciones por espacio | Readback SQL independiente: contrato sintético generado y hash conservados, futura reserva y bloqueo manual activos: PASS. Fixture no acredita emisión/firma jurídica |
| Republicar después de confirmar | Transición oculta→activa, CAS/ETag | Guard servidor para arriendo en curso/devolución pendiente; panel explícito con readback y botón Republicar, sin publicación automática | Antes de recepción PUT publication=409; confirmar por checkbox/botón, GET vacancy_confirmed=true, Republicar=200, reload activa: PASS |
| CLP y hora/día/mes | Contrato Go/OpenAPI y tarifas versionadas | Editor retenía min/step=100; corregido a min=5001/step=1 y validador >5000, sin cambiar tarifas históricas | Browser controlado: opciones hora/dia/mes exactas, 5001 válido y 5000 inválido. RED con artefacto anterior, GREEN después de build. Unitarias Go de tarifas: PASS |
| Sesiones y resultados inciertos | BFF cookie persistente y aislamiento de fachadas | Limpiar fachada y checkbox al cambiar ámbito; conservar incertidumbre hasta readback verdadero; permitir refresh sin duplicar POST | Ocho unitarias de fachada: POST/readback, error, refresh fallido/false/ausencia/espacio distinto, sesión tardía y GET competidor: PASS. Revisión de key de sesión/permisos/espacio: PASS de fuente |

## Ejecución y límites de evidencia

- Target único preservado: API limitada worker-free `127.0.0.1:18082`, PostgreSQL `127.0.0.1:15434/espacigo_f5_gate_20261101`, forward existente a `172.28.0.2:5432`. Next HTTPS `https://127.0.0.1:4203` → standalone HTTP 4204.
- Flag `LOCAL_REPUBLICATION_TRIAL=1` solo en perfil worker-free validado; no puede combinarse con LOCAL_BOOKING_TRIAL. Monta returns/operations/check-out/reception, no check-in, contratos, firma ni pagos. API conserva identidad SQL limitada; postgres solo preparó fixtures y leyó verificación.
- Dos cuentas existentes, ledger V1–V83 byte a byte y galería retenida de 30 fotos/una portada. No cuentas, credenciales o grants nuevos. Backup previo y posterior verificados fuera de tmpfs.
- Fixture final `20261104-return3`; los ensayos anteriores return1/return2 se conservan, no se hacen pasar por pruebas frescas. Preparación explícita crea únicamente arriendos/cotizaciones/ocupaciones/contrato histórico sintéticos; no ejecuta ni acredita pago, garantía, firma ni emisión contractual. Repeticiones asignan intervalos futuros no solapados y rechazan selector existente.
- `go test ./...`, `go vet ./...`, build API: PASS. Integración `TestVacancyCurrentOwnerExactIntervalAtomicRollback` con PostgreSQL real y SET LOCAL ROLE espacigo_runtime: PASS; es rollback, separado del commit nativo.
- Frontend: tipos, formato global, lint (un warning no bloqueante de img en galería), build, API check y **118 unitarias PASS / 0 FAIL / 0 SKIP**.
- Playwright global: **72 PASS / 50 SKIP / 0 FAIL**. Skips son opt-in no ejecutados en esa corrida, no fallos convertidos en aprobación.
- Playwright nativo final: **1 PASS / 0 FAIL / 0 SKIP**, `playwright.republication.config.ts`. Incluye nueve perfiles reales, otra cuenta, precondición de entrega, commit desde UI, bloqueo previo, republicación/reload, loopback y Axe desktop/mobile sin overflow. Reutiliza evidencia válida anterior de creación/galería; no afirma repetir creación/cupo completo en este test.
- El bloqueo visual inicial era un mock con IDs inválidos y wrapper data inexistente en GET espacio. Se corrigió el contrato del mock, no se debilitó el endpoint. El primer nativo encontró Origin faltante en llamadas manuales del harness: se añadió Origin real, sin modificar la protección BFF. POST reception usa `{}`; Go deriva resultado de observaciones y rechaza result enviado por el cliente.
- Readback independiente con sesión nueva: activa, devolución confirmada, 30 fotos y una portada. Capturas `images/republication-desktop-20261104.png` y `images/republication-mobile-20261104.png`; referencias privadas enmascaradas. La demo conserva datos para revisión humana, no se detiene PG/tmpfs.

## Referencia ACL independiente — diagnóstico completado

- Backup autorizado original SHA-256 `8205068b4426eb6483e8ed54b7f61f6bb12b6d4e8a3e44702741c1cdcc9952ca`, extraído con pg_restore --schema-only. Imagen exacta `postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`, PostgreSQL 180006, mismo nombre de base.
- Instancia `espacigo-acl-reference-schema-20261104`: network none, sin puertos publicados, tmpfs propio. No se copió el entorno de aplicación, credenciales de roles ni filas de cuentas. Solo se prepararon nombres de roles requeridos por el esquema, todos NOLOGIN/sin password. Postcheck **0 usuario / 0 schema_migrations / 0 roles aplicación LOGIN**.
- Restore transaccional con ON_ERROR_STOP, omitiendo únicamente CREATE SCHEMA tiger/topology porque la misma imagen ya los instala; resto de owners/ACL/extensiones se conserva. No se modificó el ensayo ni se alternó su destino.
- Snapshot READ ONLY versionado `scripts/acl_semantic_snapshot.sql`: inventario completo de 14 columnas ACL y 22 columnas owner de PG18, owners de objetos de sistema y aplicación, ACL efectiva con acldefault para NULL, PUBLIC/columnas/grant option/grantor, pg_init_privs con privtype, defaults y estado null/empty/explicit. Nombres de rol en lugar de OIDs. TOAST e índices se identifican por relación con tabla padre; no se eliminan dígitos mediante regex.
- OIDs de una misma tabla difieren entre referencia y ensayo. Comparación estricta `scripts/compare_acl_semantic.py`: **21.678 ACL efectivas, 1.513 privilegios iniciales, 9.201 owners, 12.406 estados ACL, 0 defaults; cero diferencias**. Incluye catálogos, extensiones y ACL de las bases del clúster, no solo public.
- Ocho pruebas Python PASS: grants añadidos/faltantes, columna/grant option, owners/defaults/sistema/PUBLIC, ACL/defaults vacíos, tipos, inventarios y campos OID; igualdad independiente de orden.
- Fingerprint anterior: 5.762 filas PUBLIC en ambas instancias, hashes distintos porque serializa classid/objid/grantor numéricos (`runtime_acl_f5_baseline.go:102–113`). Esto explica el fallo sin demostrar drift semántico. **No se cambió ese gate Go ni se añadió un hash observado: sigue fallando**. El diagnóstico nuevo no equivale a aprobar automáticamente el gate operacional ni sustituye membresías/flags/effective privileges de runtime.
- Referencia y snapshots privados retenidos para revisión local por el propietario; sin egress. No detener el PostgreSQL del ensayo por inferencia de limpieza Git.

## Medición de los dos displays de textura

Experimento opt-in `ESPACIGO_MEDIA_TEXTURE_QUALITY=1`, `ESPACIGO_MEDIA_TEXTURE_OUTPUT_DIR` absoluto en scratch, `go test ./internal/media -run TestCodecTextureQualityExperiment -count=1 -v`. RGB Q80/método4 de producción permanece igual.

| Fuente sintética | Q80 bytes / PSNR / SSIM global | Q70 bytes / PSNR / SSIM | Q60 bytes / PSNR / SSIM |
|---|---|---|---|
| exterior_texture JPEG | 1.044.618 / 22,1778 dB / 0,992600 | 885.962 / 21,8839 / 0,988491 | 811.982 / 21,6738 / 0,985631 |
| webp_texture | 1.190.300 / 29,0421 dB / 0,997315 | 1.030.414 / 27,8530 / 0,995213 | 955.794 / 27,1020 / 0,993442 |

Medidos también encode_ms y dimensiones/metadatos en el log del experimento. Los crops de referencia/Q80/Q70/Q60 se inspeccionaron: Q60 suaviza textura fina; las líneas gruesas siguen visibles. Ninguna candidata cumple 614.400 bytes. El SSIM es global, no una aceptación perceptual ni prueba de fotografía representativa. **Sin ajuste de encoder**: sigue pendiente corpus fotográfico autorizado y piso visual antes de decidir dimensiones/peso/calidad. No inventar aprobación humana ni cambiar la meta para declarar cierre.

## Reproducción y pendientes concretos

1. Preservar la política/inventario privados y verificar target/servicios; no copiar credenciales a referencia o repositorio. API limitada con flag anterior, Next standalone reconstruido, proxy TLS loopback retenido.
2. Para un nuevo ensayo autorizado, `python scripts/prepare_republication_trial.py --suffix YYYYMMDD-selector --inventory <archivo privado existente> --confirm local-synthetic-republication-only`. No reutilizar un selector ni crear cuentas. Preparación no es aceptación del recorrido.
3. En frontend, con PHASE_REPUBLICATION_NATIVE_E2E=1 y rutas privadas PHASE6_PRIVATE_TESTING_DIR, PHASE6_ACCOUNT_INVENTORY_FILE y PHASE_REPUBLICATION_FIXTURE_FILE, ejecutar `./node_modules/.bin/playwright test --config=playwright.republication.config.ts`. Evidencias privadas desactivadas: trace/video/screenshots off.
4. Pruebas controladas usan el comando start:local del package.json mediante un launcher acotado cuando pnpm no está en PATH; no se instaló ni sustituyó una versión del gestor. Sin mocks en el nativo.
5. Pendientes fuera de este cierre funcional de Mis espacios: corpus/piso visual de textura; adaptación revisada del gate operacional ACL (no un tercer hash); lectura/revisión documental por roster autorizado y cambios de reserva/emisión/firma/cobros/garantía en los cortes que fija el ADR; F7/cloud, main y aceptación humana separados. No repetir ni invalidar el cierre documental histórico por estos pendientes.
