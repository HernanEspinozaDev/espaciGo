# Reconciliación de espacios con main — 2026-11-06

## Estado y alcance

Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-release`, rama autorizada `codex/marketplace/feature/release-83-espacios`. Merge frontend cerrado en `d2487324084cc5840388991ab028fbd68756065e`, padres `9edd61c405bc01f7f5ac5c46e616a6d3063bdf5f` y `7a9dc8244f82277c8d303d14a57be21b68638c4d`; incorporación normal de la rama DB cerrada en `cd517412432a78b2702c0066dc41e95858a2cabe`. Ese checkpoint conserva el frontend y OpenAPI byteidénticos al merge frontend probado y las migraciones byteidénticas a la base. El recibo JSON contiguo fija SHA256 de las fuentes críticas probadas. El agente padre es el único dueño del build. Es checkpoint de integración local, no release cloud documental listo.

Cloud SQL ya quedó V83/pending0 el 2026-11-05; entrega DB publicada en `codex/database/fix/cloudsql-83-espacios`, HEAD `61ecc92b51eceaf22fd7275c1bd3f8c209e075ab`, ahora incorporada como segundo padre del checkpoint. No se reaplicó SQL durante esta continuación. Main conserva `f4bc7bd2330c91827a02b01aeef71209dff12b01`: sin push ni despliegue.

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

Se detectó que la fachada había perdido cambios combinados no comprometidos; la causa exacta de la sustitución no quedó acreditada y no se atribuye a una hoja sin evidencia suficiente. El padre recuperó por AST los cuerpos desde un módulo realmente ejecutado antes del contador, conservó las firmas TypeScript y comprobó emisión JavaScript byteidéntica a ese artefacto. Después repitió RED/GREEN del contador, casos de éxito/error tardío y las regresiones de mutaciones. El cierre global anterior es posterior a esa recuperación. No hubo reset, pérdida de commits ni alteración de la historia. El snapshot final de fuente queda retenido como `.ts.txt` privado; no se presenta una elección ours/theirs como integración.

## Revisión y cierre de los merges

La revisión funcional independiente `deleg_4ccdd6aa` no encontró bloqueantes demostrables en fachada/bridge/cola/UI; reportó28pruebas focales, no browser/build/DB nuevos. La revisión de seguridad `deleg_4f21514a` devolvió JSON válido `passed:true` con listas vacías de riesgos/errores concretos, dentro de su cobertura declarada de auth/BFF/documental/galería y los seis candidatos `new Function` de tooling. Estos ejecutan código versionado con fixtures, no input arbitrario de runtime. No revisó cada línea de los120archivos.

La observación de hashes de esa revisión quedó resuelta antes de comprometer: el padre reprodujo exactamente `999eb82…` con `git diff --cached --unified=0`, `c2c17af…` con contexto por defecto y `59e7336…` con `--binary`. Son representaciones diferentes del mismo índice, no un cambio de fuentes. Los tres hashes completos se conservan en el recibo. Diez fuentes críticas siguieron coincidiendo con su digest y el índice quedó sin conflictos.

El merge DB solo tuvo conflicto documental en el plan: se combinó la autorización/estado vigente, los gates449/139 y la fila DB con su evidencia detallada, manteniendo una sola matriz. El procedimiento canónico conservó su historial y añadió el registro V83. Tras reunirlo se ejecutó de nuevo `go test ./...` en entorno allowlist: PASS, con caché/opt-in nativo no habilitado (`spaces-release-post-db-go.log`). `git diff` confirma frontend/OpenAPI sin cambios respecto de `d248732` y V1–V83 sin cambios respecto de `9edd61c`; no se repitió innecesariamente la suite browser idéntica.

El borrador rechazado preV81 y su comprobación estática se conservaron byte por byte como `.txt` bajo `planning/gcp/rejected/`, fuera de SQL operativo y descubrimiento de tests. No sustituyen los repairs V81/V82/finalV83 realmente revisados/aplicados.

## Gates restantes

- Publicar/verificar el checkpoint en su rama de tarea y continuar la adaptación documental cloud segura. Los dos merges ya están cerrados conservando sus padres e historia; no hay aprobación humana de producto inventada.
- Documental cloud seguro sigue sin store de objetos cifrados, scanner privado y worker durable desplegados. Sus recursos, IAM, clave, red y coste requieren la operación concreta autorizada; no trasladar excepciones F5, crear cuentas SQL ni relajar permisos para avanzar.
- Push a main dispara `espacigo-main-rollout`; no ejecutarlo por inferencia de este PASS. Mantener los gates del procedimiento/autoridad de lanzamiento y la compatibilidad del SHA exacto.
- Las demos y sus cuentas, datos, volúmenes y backups se conservan. Esta evidencia es regresión local controlada y de código, no una nueva aceptación humana ni recorrido funcional online.
