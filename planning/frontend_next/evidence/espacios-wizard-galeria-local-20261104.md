# Wizard y galería — entrega local de creación de espacios

Fecha local: 2026-11-04 (-03). Recorrido funcional local verificado; no aceptación humana de producto, integración a main ni despliegue.

## Alcance y base

- Usuario autorizó wizard, galería visible autenticada y publicación elegible en el ensayo sintético existente; sin main, despliegue ni F7.
- Backend: rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`, base `93b59c8352960d900b4e62bb42bced9e05dcad8c`.
- Frontend: rama `codex/frontend/fix/local-close-02-f6-native-e2e`, base `cc2fdc39670477b5c051ea74c853bb905604fb4d`.
- Subagentes lanzados por CLI explícitamente con `--model gpt-6-luna --provider openai-codex --reasoning medium`, archivos de propiedad exclusivos; padre integra y posee build/browser final.
- Se conserva evidencia previa de borrador persistido, F3, galería backend F4 y F5-A3/F6 documental. No repetir cuentas/migraciones/documentos para presentar avance nuevo.

## Recuperación del mismo ensayo

Preflight encontró PostgreSQL detenido desde `2026-11-04T19:31:44.482104608Z`, con `/var/lib/postgresql` tmpfs. No se sustituyó target. Nueva confirmación humana: «Autorizar recuperación exacta desde el backup final».

Backup final `f5-f6-post-browser-20261104T083403Z.dump`, SHA-256 `8205068b4426eb6483e8ed54b7f61f6bb12b6d4e8a3e44702741c1cdcc9952ca`, revalidado antes del restore. Arrancó exclusivamente el mismo contenedor; forward propio loopback15434 a172.28.0.2:5432. Identidad READ ONLY PostgreSQL18.6, postgres, DB exacta `espacigo_f5_gate_20261101`; ledger y roles aplicación ausentes.

Runner revisado `f5-role-recovery-runner.py` + SQL versionado SHA-256 `66e1fc49a6206c29ef1a0c0fa07b3881be6ced3199bc6b2f43577f5e1deb2f47`: preview rollback PASS, readback cero roles; aplicación roles previa al restore, flags seis roles/tres memberships mínimos y autenticación ambos logins con passfiles existentes PASS. Sin reset/copia/publicación de credenciales.

Restore transaccional del backup final: exit0, owners/ACL conservados; TOC excluye únicamente CREATE SCHEMA tiger/topology ya instalados. Ledger restaurado V1–V83 con83/83 nombres/checksums exactos; no se ejecutó migración ni replay ACL. Readback confirmó las dos cuentas del inventario final recuperadas, sin registro ni verificación adicionales.

La comparación schema-only completa archive→destino restaurado, con `normalize` del checker versionado (solo metadata externa pg_dump), devuelve igualdad exacta de SQL/owners/ACL. SHA-256 normalizado `ecf2aa2ade29c828d46eb00937cf2d7fe7a72d60130cd60115cb8f28e363cea1`. **El test histórico de ACL por OIDs no pasa en este nuevo cluster**: devuelve fingerprint distinto. No se aprende hash, no se cambia guarda ni se ejecutan grants para hacerlo pasar. Esta limitación del fingerprint se separa de la equivalencia estructural exacta verificada y del recorrido runtime aún pendiente.

API propia compilada de la base backend, perfil real worker-free en127.0.0.1:18082, login limitado API→SET ROLE runtime. Ready200; login sintético200, listado espacios200 y categorías200. El catálogo `/public/locations` no está disponible en este runtime (404 sin query/400 con query), por tanto no se simula catálogo completo de comunas ni se promete resolución de nombres. No se arranca scanner para esta galería: la conversión WebP es media Go y el scanner documental previo conserva su evidencia histórica.

## Discrepancia de árboles detectada

El frontend de la rama de tarea conservaba editor de fotos sintéticas y adaptador sin CAS de galería. El backend contiene el código UI/galería posterior ya documentado en F4. Se asignó adaptación selectiva de ese trabajo a la rama frontend, preservando las correcciones auth/POST spaces y sin reemplazar el wizard concurrente. El primer ejecutor E2E no entregó spec y afirmó incorrectamente límite10; padre verificó `internal/gallery/domain.go` y `internal/media/domain.go`, ambos30, y reasignó preparación con referencias absolutas y obligación de implementar.

## Medición peso/calidad repetida

`go test -count=1 -v ./internal/media -run '^TestConvertSyntheticScenarioCorpus$'` PASS, seis escenarios sintéticos (4.827s). No es aceptación visual ni corpus fotográfico representativo.

| Escenario | Display bytes | Thumbnail bytes | Meta display≤614400 / thumbnail≤102400 |
|---|---:|---:|---|
| interior_edges | 18042 | 4588 | Cumple ambas |
| exterior_texture | 1044618 | 98496 | Display no cumple; thumbnail sí |
| portrait_orientation | 19148 | 4972 | Cumple ambas |
| transparent_plan | 1934686 | 176864 | No cumple ambas |
| small_image | 846 | 846 | Cumple ambas |
| webp_texture | 1190300 | 91266 | Display no cumple; thumbnail sí |

Prueba verifica WebP decodificable, dimensiones máximas1600/480 sin ampliar entradas pequeñas, orientación EXIF6 y ausencia estructural de EXIF/XMP/ICCP/animación. Calidad80 y dimensiones no cambiadas: falta decisión/piso visual y corpus representativo para ajustar sin degradación arbitraria; no se convierte SLO en rechazo automático. No bloquea los demás pasos funcionales.

## Pendientes separados del recorrido de esta entrega

- Republicación posterior a entrega/desocupación, conservación de historial e intervalos: corte transversal ADR, no acreditado por publicar un borrador elegible.
- Lectura binaria por revisores y revisión manual visible: contrato/acceso explícito y ensayo independiente; la revisión/revocación backend sintética ya probada no implica esa UI.
- Contratación/cobros: identidad al emitir, emisión/aceptaciones, bloqueo/vencimiento, renta, firma opcional y garantía por etapas. Depósito sigue deshabilitado.
- F7/cloud/main/despliegue fuera de esta autorización.

## Resultado reunido y matriz final

Última corrida nativa: fixture **nuevo** `20261104-end2204`, `playwright.space-gallery.config.ts`, **1 PASS, 0 FAIL, 0 SKIP**. El spec rechaza reutilizar un selector de creación ya existente. Se ejecutó sobre standalone reconstruido con todas las correcciones de producción reunidas, HTTPS loopback4203 → Next4204 → Go18082 → DB15434. No hubo mocks ni SQL de escritura en el recorrido.

| Criterio de esta entrega | Resultado y evidencia |
|---|---|
| Comuna y resumen honestos | Código `13101`/región `13`; negativos de formato/coherencia; sin catálogo inventado ni promesa/header de idempotencia en POST spaces. |
| Crear borrador, recargar, editar, volver a recargar | POST201 real, configuración zona200, PUT200 y título rehidratado. Sesión BFF restaurable con cookie Secure/HttpOnly; el token opaco no sale en JSON ni requests del navegador. |
| Subir y rehidratar fotografías | Tres PNG válidos sintéticos por UI; galería y variantes privadas reales. Tras reload:30 tarjetas y todas las imágenes decodificadas (`naturalWidth>0`). |
| Portada, orden, reemplazo y retirada | Readbacks independientes después de cada acción. Reemplazo inválido rechazado preserva DTO completo de la foto anterior, portada y posición; reemplazo válido201, confirmación de retirada y DELETE200 verificado ausente. Se selecciona una portada sobreviviente. |
| Cupo | Preparación repetitiva por BFF real hasta30; foto31 recibe409 y permanece30. No se usan SQL ni fixtures simulados para llenar el cupo. |
| Publicar elegible | Botón UI, PUT publication200, lectura nueva confirma `activa`. |
| Ownership | Otra cuenta sintética existente recibe403/forbidden sin items al leer galería. Ambos logouts204. |
| Pending202 y fallos ambiguos | Regresiones ejecutables: pending/deleting no son éxito ni habilitan replay; lookup por misma clave, ready necesita readback, retry explícito conserva File/key/target/ETag. Mutaciones competidoras bloqueadas mientras incierto. 422 validation_error es rechazo definitivo del codec; permite otro archivo sin perder foto anterior. Cobertura simulada/handlers, no se afirma haber provocado202 real en Go. |
| Respuestas tardías | Fachada y componente rechazan resultados obsoletos tras clear/selección/desmontaje. No publican portada/orden/retirada optimistas antes del readback; fallo de lectura conserva snapshot y declara resultado sin confirmar. |
| Accesibilidad/visual | AXE del panel real con30 fotos sin violaciones, desktop1365×900/mobile390×844, sin overflow. Corregido contraste del botón Publicar y Eliminar sobre portada. Capturas recortadas inspeccionadas: solo imágenes sintéticas, sin credenciales/correos/UUIDs. No equivale a aceptación visual humana. |

### Gates ejecutados

- `node --test tooling/*.test.mjs`: **100 PASS, 0 FAIL, 0 SKIP**.
- `tsc --noEmit`, ESLint, Prettier global y build producción: PASS. ESLint mantiene dos warnings no bloqueantes del componente (ref de generación y uso de img privada); cero errores.
- `node tooling/api-check.mjs`: tipos/allowlist/inventario vigentes,187 operaciones.
- Playwright global controlado: **69 PASS,49 SKIP,0 FAIL**. Skips son gates opt-in separados (incluido nativo), no integración atribuida. Log `gallery-controlled-final3.log`.
- Playwright nativo fresco completo: **1 PASS**, log `gallery-native-acceptance.log`; incluye AXE del panel con fotografías en ambos viewports.
- Readback posterior independiente: activa,30 fotos, exactamente una portada; todas las imágenes decodifican; AXE desktop/mobile PASS. Runner local `gallery-demo-readback.mjs`, ruta precisa solo en `.local/testing/space-gallery-demo/review-target.json` (0600).
- Backend `env -i ... go test -mod=readonly -timeout=300s ./... -count=1`, `go vet -mod=readonly ./...`: PASS, sin DSN; no se repite ni atribuye suite documental live. Checker schema12/12 PASS.
- Revisión fuente independiente GPT-6 Luna medium, sesión `local-cf665180159ae72e91723d78542191855059491ed487746a5b7e1386935e1e6a`: PASS sin hallazgos; límites source-only explícitos. Padre inspeccionó diff y ejecutó los gates anteriores. Dos ejecutores anteriores no completaron su corrección; el padre corrigió y verificó, no se contó exit0 como aceptación.

### Fallos preservados y resolución

- Global inicial58PASS/49SKIP/11FAIL: mocks8/9 tenían expiración fija ya vencida; ahora fija por creación de fixture y no renovada en cada request. El guard de red trataba URLs blob privadas de loopback como red externa; se comprueba su origen interno, no se elimina la guarda. AXE detectó contraste real insuficiente; corregido y vuelto a probar.
- Primer nativo nuevo: carrera del test al seleccionar archivo antes de terminar el reordenamiento; `setInputFiles` no espera controles enabled. Ahora se exige enabled antes de adjuntar. Retirada acepta explícitamente el confirm y comprueba commit por lectura independiente.
- Selector antiguo `final6` reutilizaba un borrador anterior sin zona configurada; publicación se bloqueó correctamente. No se habilitó artificialmente. Gate final exige selector fresco y ejecutó alta/configuración de zona desde UI.
- El fingerprint ACL histórico dependiente de OIDs de esta restauración sigue sin PASS; no se cambió baseline, ACL ni gate. La equivalencia estructural exacta del backup/restauración y el comportamiento runtime se registran por separado.

## Respaldo y demo retenida

- Base exacta y V1–V83 revalidadas read-only:83/83 nombres/checksums idénticos a fuentes. Sin migraciones, grants, cuentas nuevas ni resets.
- Backup fuera de tmpfs: `gallery-accepted-20261105T010447Z.dump`,717956 bytes, SHA256 `5b0539408db4789dc423bcc2a83bd573b256cd46b2aa88edeacb9f3c3a4c2257`. TOC y decodificación completa pg_restore a `/dev/null` PASS (no restore ni prueba de recuperación en otro target).
- Storage local multimedia/documental: `gallery-storage-accepted-20261105T010447Z.tar.gz`,110189 bytes,852 entradas, SHA256 `27b668c0cd0cf6cf1b466faf5fdfd63d39970ec7ecbcf907498ee0d88967052c`; lectura integral del archive PASS. Ambos bajo `/home/nandev/.local/share/espacigo-f5-backups/`,0600. Timestamp UTC05 corresponde a noche local04.
- API/Next/proxy/forward propios quedan retenidos para revisión del usuario en `https://127.0.0.1:4203/anfitrion/espacios`. Inventario autorizado existente bajo `.local/testing/`, nunca en capturas/documentos versionados. Responsable de retención: usuario para revisión de demo; no es limpieza ni aceptación final. PostgreSQL tmpfs no debe detenerse; backups externos conservados. Scanner/Mailpit detenidos no se reiniciaron para esta galería worker-free.
- Frontend probado y publicado: `bd4b940065037d442f6f15e96a18261695be0b95`, rama `codex/frontend/fix/local-close-02-f6-native-e2e`. `git ls-remote` coincide con HEAD, worktree limpio, hook e identidad HernanEspinozaDev verificados. Backend conserva código API probado `93b59c8352960d900b4e62bb42bced9e05dcad8c`; el commit posterior contiene únicamente contrato OpenAPI de portada y documentación/capturas. SHA backend final en recibo tras readback remoto; ningún push a main, PR, F7 o despliegue.

## Límites de fase que permanecen

El recorrido solicitado queda verificado; no se cierra el plan global por inferencia. Las metas de peso/calidad de §medición siguen parciales (sin decisión de piso visual/corpus representativo). Esta UI serializa una fotografía por vez, no acredita cola2 completa. Republicación, lectura binaria documental de revisores y contratación/cobros permanecen como cortes separados de §pendientes. La demo de30 fotos no afirma publicación pública de bytes, uso productivo ni datos reales.
