# Reconciliación de espacios con main — 2026-11-06

## Estado y alcance

Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-release`, rama autorizada `codex/marketplace/feature/release-83-espacios`, HEAD base `9edd61c405bc01f7f5ac5c46e616a6d3063bdf5f`, merge frontend abierto de `7a9dc8244f82277c8d303d14a57be21b68638c4d`. Las pruebas acreditan el working tree combinado, no un commit candidato limpio. El recibo JSON contiguo fija SHA256 de las fuentes críticas probadas. El agente padre es el único dueño del build.

Cloud SQL ya quedó V83/pending0 el 2026-11-05; entrega DB publicada en `codex/database/fix/cloudsql-83-espacios`, HEAD `61ecc92b51eceaf22fd7275c1bd3f8c209e075ab`. No se reaplicó SQL durante esta continuación. Su incorporación por Git normal sigue pendiente de cerrar el merge frontend. Main conserva `f4bc7bd2330c91827a02b01aeef71209dff12b01`: sin push ni despliegue.

## Correcciones verificadas

- Readbacks simultáneos de fotos: generación de selección y contador de lectura, sin usar identidad mutable del objeto seleccionado. Dos cargas legítimas terminan sin falsa incertidumbre; A/B/A y respuestas fuera de orden no publican fotos, revisión o errores antiguos. Los identificadores del contador se purgan al limpiar sesión.
- Upload/replacement/delete/cover/move: un resultado ambiguo o abort después del envío bloquea nuevas mutaciones hasta GET autoritativo. Un 422 definitivo no se presenta como commit incierto. 409/412 requieren refresh. Resultados tardíos A/B/A no contaminan la selección nueva. 401/403 purgan sin repoblar error o incertidumbre, incluido commit de portada seguido de denegación durante readback; la causa de autorización no queda escondida en un error genérico.
- Lookup terminal reconcilia fotos y revisión antes de permitir un reintento deliberado, con el mismo File/key y, para replacement, el If-Match original. Pending/deleting no permiten replay ni descarte. Not_found no demuestra cancelación: solo un failed autoritativo posterior habilita el descarte local y la liberación del File.
- Replacement202 conserva los objetos/fotografía anteriores hasta confirmación terminal, evitando renovar el Blob del contenido aún vigente. Se restauró la prevalidación de extensión/MIME/tamaño que ya tenía main, sin añadir conversión en navegador.
- La primera portada conserva la intención del primer archivo encolado aun cuando llegue pending. Usa CAS y GET, una sola tentativa de metadato, callbacks estables y descarte de continuaciones desmontadas. Una portada encontrada por refresh no se sobrescribe. Si el metadato queda incierto, la foto confirmada puede contarse como agregada, pero la UI conserva la cerca y permite refresh explícito sin reupload.
- El bridge completa la revisión seleccionada al publicar/archivar cuando la UI no proporciona una explícita; nunca reemplaza una revisión explícita de la intención. Se reprodujo RED tanto en navegador como en pruebas ejecutables del bridge y se verificó GREEN.
- Harness reconciliados con el DOM y DTO Go actuales sin retirar CAS, límites30, cancelación queued sin POST, retención del archivo/key, rechazo, recuperación, portada/reorder ni publicación/contenido posterior. El control de red reconoce el origen embebido de Blob locales y rechaza externos. La suite documental fija la expiración de sesión por fixture, en vez de renovarla artificialmente en cada GET; el ensayo de entregas cubre todos sus GET con respuestas locales explícitas.

## Ejecución del padre

Todas las comprobaciones browser se ejecutaron con Playwright aislado en `http://127.0.0.1:4217`, entorno allowlist sin credenciales SQL/API ni flags nativos, sobre standalone recién construido. No se usó el navegador del usuario ni la aplicación online.

- `pnpm api:check`: PASS,197 operaciones y generados actuales.
- `pnpm typecheck`, `pnpm format:check`: PASS.
- `pnpm lint`: cero errores; tres warnings previos (img privado y dos bindings de tests).
- `pnpm test`: **449 PASS / 0 FAIL / 0 SKIP**.
- Build de producción: PASS.
- Suite completa `pnpm exec playwright test --workers=2`: **139 PASS / 0 FAIL / 54 SKIP**,193 casos recogidos. Los54 nativos opt-in no se habilitaron; no constituyen éxito nativo.
- Focal de creación/galería/incertidumbre:25PASS. Incluye nueve perfiles, CLP5001 y unidades hora/día/mes, publicación y edición posterior, cola/cancel/retry y primera portada pending.
- `go test ./...` con entorno allowlist: PASS. Incluye resultados cacheados; no acredita una nueva integración PostgreSQL opt-in.

Logs privados sin credenciales bajo scratch: `spaces-resume-unit-final.log`, `spaces-resume-build-final.log`, `spaces-resume-full-e2e-final.log`, `spaces-resume-go.log`. La primera corrida global falló en dos fixtures; se conservan sus logs y se verificó la corrección focal4PASS antes de repetir el conjunto.

## Incidente de ejecución y recuperación

Una hoja de la tarea de cerca de mutaciones sustituyó el working tree de la fachada por un stage del índice, violando su ownership y perdiendo cambios combinados no comprometidos. Esa entrega se rechazó. El padre recuperó por AST los cuerpos desde un módulo realmente ejecutado antes del contador, conservó las firmas TypeScript y comprobó emisión JavaScript byteidéntica a ese artefacto. Después repitió RED/GREEN del contador, casos de éxito/error tardío y las regresiones de mutaciones. El cierre global anterior es posterior a esa recuperación. No hubo reset, pérdida de commits ni alteración de la historia. El snapshot final de fuente queda retenido como `.ts.txt` privado; no se presenta una elección ours/theirs como integración.

## Gates restantes

- Adjudicar la revisión independiente del árbol actual y completar la resolución semántica/índice del merge; después incorporar la rama DB conservando historia.
- Documental cloud seguro sigue sin store de objetos cifrados, scanner privado y worker durable desplegados. Sus recursos, IAM, clave, red y coste requieren la operación concreta autorizada; no trasladar excepciones F5, crear cuentas SQL ni relajar permisos para avanzar.
- Push a main dispara `espacigo-main-rollout`; no ejecutarlo por inferencia de este PASS. Mantener los gates del procedimiento/autoridad de lanzamiento y la compatibilidad del SHA exacto.
- Las demos y sus cuentas, datos, volúmenes y backups se conservan. Esta evidencia es regresión local controlada y de código, no una nueva aceptación humana ni recorrido funcional online.
