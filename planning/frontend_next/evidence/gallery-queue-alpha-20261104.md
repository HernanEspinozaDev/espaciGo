# Continuación local: cola2 y alfa — 2026-11-04 (-03)

Base frontend `bd4b940065037d442f6f15e96a18261695be0b95`, rama `codex/frontend/fix/local-close-02-f6-native-e2e`; backend base `ae09d5d5b1627bfcba1a4cdfac76853eceed8ab8`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`. Sin main, PR, despliegue ni F7.

## Entregado y verificado

- Selección múltiple con motor independiente y máximo2 solicitudes simultáneas. Un POST por archivo, mismo File/UUID de intención al reintentar explícitamente. Queued cancelado no despacha ni suma éxitos.
- AbortSignal llega por componente→hook→fachada→adaptador. Abortar envío no prueba rollback: lookup por clave después de settle, pending/deleting sigue incierto, failed/not_found solo habilita retry explícito. Consultas manuales deduplicadas y con fence de vigencia. Ready requiere readback; dispose aborta y no publica callbacks tardíos.
- Estado accesible por elemento, sin nombres privados; resumen cuenta solo ready. Cuota de selección30 y mutaciones/publicación bloqueadas mientras hay cargas sin resolver. La sesión BFF y los CAS del cierre anterior se conservan.
- Codec backend mantiene calidadRGB80/método4; fija compresión alfa y calidad alfa100. Regresión RED real alfa7→0 y salida sin compresión1.9MB; GREEN alfa exacto, bytes opacos iguales y corpus transparente1934686/176864→14862/4142bytes.

## Ejecución del padre

| Gate | Resultado |
|---|---|
| Unitarias globales |110PASS/0FAIL/0SKIP;8 del motor de cola |
| TypeScript, ESLint, formato, build producción |PASS;1warning img privada,0errores |
| OpenAPI/inventario generado |PASS,187 operaciones |
| Playwright controlado global |71PASS/49SKIP/0FAIL |
| NativoHTTPS Go/DB/storage |1PASS/0FAIL/0SKIP, fixture nuevo20261104-queue3 |
| AXE panel real desktop/mobile y overflow |PASS con30fotos |
| Go completo/vet sin DSN y experimento codec opt-in |PASS; no equivale a documental live ni ACL live |

Native: `PHASE_SPACE_GALLERY_NATIVE_E2E=1 PHASE_SPACE_GALLERY_FIXTURE_SUFFIX=20261104-queue3`, inventario privado autorizado existente, `playwright test --config=playwright.space-gallery.config.ts`. El selector se consumió; usar otro fresco al repetir. Tres archivos juntos/máximo2POST,3respuestas201/readback3ready; uno RGBA y canvas de las variantes reales devuelve `[96,255,255]`. Después recorrido completo editar/reload, portada/orden/reemplazo fallido conserva anterior/reemplazo válido/retirada, cupo30/31, publicaciónactiva y otra cuenta403/logouts204.

Dos nuevos E2E mock, separados del nativo, prueban cancelación queued4.º/3POST, despacho del3.º solo tras liberar slot, resumen3ready/1cancelled, selección excedida sinPOST y202→lookup→retry con misma clave/bytes. El primer test falló por locator alert ambiguo con anunciador Next; acotado al region de cola. El primer probe alfa dependía de postData multipart no expuesto; reemplazado por canvas real y repetidoPASS. No se ocultaron fallos de producto ni se relajaron gates.

Re-review fuente GPT-6 Luna medium `local-9891181c28fefdecef72280236659d30d52299beeb4a0cd3821d1f35d3947089`: PASS sin hallazgos residuales. Padre ejecutó gates; la revisión no certifica sus conteos por sí sola.

## Demo/límites

Demo retenida `https://127.0.0.1:4203/anfitrion/espacios`; ruta específica de queue3 solo en `.local/testing/space-gallery-demo/review-target.json` del backend. Sin nuevas cuentas/contraseñas/migraciones. Base/volumen/secretos ajenos conservados. Backup externo dump `gallery-accepted-20261105T013910Z.dump` SHA256 `be93c2b379087e6e37a0842484ffefd707be2c388689de9ab0c30dc84d2bd82e`; storage `gallery-storage-accepted-20261105T013910Z.tar.gz` SHA256 `e3b6b9b21770e72e06bd3db7e47e154f33f67ec29e423d7391c3003783e61a05`. TOC/parse completo y lectura integral de storagePASS, no restore nuevo. Readback posterior activa/30/una portada y ledger83checksums exactos. Recursos propios retenidos para revisión del usuario.

Evidencia detallada, corpus, revisión ACL y capturas en rama backend: `planning/frontend_next/evidence/galeria-cola-alpha-20261104.md`. Quedan2metas display de textura (1044618/1190300frente614400), no se baja calidad sin corpus/piso visual; ACL read-only sigueFAIL por fingerprintOIDs y requiere referencia semántica independiente con cobertura de sistema. Sin tercer hash aprendido ni grants. Republicación, lectura por revisores y contratación/cobros siguen separados. SHA finales publicados en recibo tras verificar refs; no aceptación humana inventada.
