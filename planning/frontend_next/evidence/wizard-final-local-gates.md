# Gates locales definitivos — mejoras del wizard

## Cierre local actualizado antes de publicación de código

La extensión pública fue autorizada después del primer conjunto de gates de este documento. Su publicación local normal pasó con un único PUT de UI/CAS y recibo verificado; el contexto anónimo leyó el DTO real y la ficha SSR. PostgreSQL read-only confirmó exactamente una auditoría del propietario y exclusión del borrador día. La reconciliación pública posterior pasó sin PUT. Véase `wizard-native-public-positive.md`.

Checks de cierre repetidos: API check, TypeScript, lint, formato y unitarias frontend PASS (477 PASS/1 SKIP); browser conjunto **145 PASS/63 SKIP**. Los 63 omitidos no se convierten en nativos aprobados. Go global fresco y vet PASS tras el delta público; además los dos casos PostgreSQL explícitos pasaron. Python combinado de scripts: **47 PASS**; cuentas/verificación: **14 PASS**; legacy CLI/import: **2 PASS ejecutados directamente**, sin pytest. La prueba de puerto ocupado de Mailpit usa ahora mocks, no intenta bind al puerto del forward retenido. El postcheck confirmó que ninguna suite creó un journal falso en el ensayo real.

La revisión independiente de frontend/backend y la revisión posterior de correcciones operativas/opt-in público concluyeron sin bloqueadores concretos. Eso acredita revisión de código dentro de sus alcances, no aceptación humana. Los fallos y su adjudicación se preservan en `wizard-final-review-adjudication.md`.

Las cuatro capturas regionales finales revisadas se copiaron byte a byte a `.local/testing/wizard-improvements/review-artifacts/`, directorio 0700/archivos 0600 con manifest de hashes, para no depender de la poda de caché. No contienen formularios de acceso ni identificadores de cuenta. Se retienen junto al ensayo por el ejecutor para revisión/reproducibilidad; no se limpian datos o credenciales por inferencia.

El conjunto local está listo para commit/publicación de rama, pero **no declara cierre de entrega**. `origin/main` avanzó a `214c75dc612374c868cc4209ecef521b497d0c19`: antes de integrar se deben conservar sus cambios de release sin bastión y conciliar el CLI migrador, repetir los gates afectados y seguir su orquestador vigente. No hay push/main/rollout de este conjunto todavía. Las secciones inferiores conservan el primer corte probado y sus límites históricos.

## Identidad del conjunto probado

- Worktree: `/home/nandev/Documentos/projects/espaciGo-wizard-improvements`.
- Rama: `codex/frontend/fix/mejoras-creacion-espacios`.
- HEAD base: `771134d165046b290b1f5ad3a1074a6357c2327f`; cambios locales todavía sin commit. No atribuir la implementación a ese SHA base.
- Next build ID: `3vVei_2oiWumIiG9a51RW`.
- Binario API construido desde fuentes actuales: SHA-256 `ed7a4d4ce9538f455979f77e9d680c24c59deb6ea6b4ec9178d2d40e45810579`, retenido bajo el directorio privado del ensayo, no en caché expirable.
- Manifiesto local de 111 archivos cambiados al congelar la revisión: `/home/nandev/.hermes/cache/scratch/wizard-final-source-manifest.json`. Se verificó después que los digests de archivos no documentales no habían cambiado. Las actualizaciones posteriores de evidencia no son cambios de producto.

## Resultados ejecutados por el padre

| Comando / recorrido | Resultado | Capa y límite |
| --- | --- | --- |
| `pnpm api:check` | PASS; 197 operaciones | Tipos, allowlist e inventario generados actuales |
| `pnpm typecheck` | PASS | TypeScript global |
| `pnpm lint` | 0 errores, 4 advertencias | No se suprimieron reglas |
| `pnpm format:check` | PASS | Formato global frontend |
| `pnpm test` | 477 PASS, 1 SKIP, 0 FAIL; 478 total | Unitarias/harness frontend, no integración nativa |
| `pnpm build` | PASS | Artefacto de producción actual; assets standalone sincronizados |
| `PLAYWRIGHT_PORT=4216 pnpm exec playwright test --workers=2 --output=…/global-definitive` | 145 PASS, 61 SKIP | Suite conjunta; los omitidos no se presentan como nativos aprobados |
| `pnpm exec playwright test --config=playwright.public-calendar.config.ts --output=…/public-definitive` | 2 PASS | SSR público contra fixture controlada; no API pública real |
| `ESPACIGO_WIZARD_NATIVE=1 … playwright test --config=playwright.wizard-native-visual.config.ts` | 1 PASS | Editor real, ambos viewports, Axe completo, contención y cero mutaciones de espacio observadas |
| `ESPACIGO_WIZARD_NATIVE=1 … playwright test --config=playwright.wizard-native.config.ts --workers=1 --grep 'reconciles the saved'` | 1 PASS | Editor real, recarga, CAS 412 y readback intacto; sin repetir guardado |
| Prueba Go `TestWizardNativeDetailReadOnlyDraftExcludedFromPublicCatalog` con opt-in y passfile aprobado | PASS | Proyección completa PostgreSQL read-only y exclusión del borrador; véase evidencia específica |
| `go test -count=1 ./...` | PASS | Ejecución global Go sin atribuirle los gates nativos omitidos |
| `go vet ./...` | PASS | Análisis Go global |
| `git diff --check` | PASS en el conjunto probado | No prueba por sí solo los archivos nuevos ni seguridad |

Logs: `/home/nandev/.hermes/cache/scratch/wizard-final-{api-check,types,lint,format,unit,build,browser,public-browser,go,go-vet}.log`.

## Corrección de franjas parciales y revisión visual

El calendario conserva el cierre exacto 17:30 y diferencia la celda 17:00–18:00 como **Parcial**, con estado accesible `mixed`. Las horas completas permanecen seleccionadas y las vacías no lo están. El helper reconoce también el hueco inicial de 09:15–10:00 y 23:30–24:00, sin redondeo, mutación ni pérdida de fragmentos.

El primer build con la etiqueta parcial produjo `label-content-name-mismatch` en Axe. Se corrigió el nombre accesible derivándolo del contenido visible y conservando el detalle de tramos en texto sólo para lectores de pantalla. Se reconstruyó antes de repetir los gates nativos y la suite conjunta. No se excluyó la regla ni se redujo la auditoría.

Capturas BEFORE preservadas byte a byte bajo `…/espacigo-wizard-evidence/native-editor-visual-before-partial/`. AFTER bajo `…/espacigo-wizard-evidence/native-editor-visual/`, archivos `native-editor-schedule-desktop.png` y `native-editor-schedule-mobile.png`. El padre abrió ambas imágenes AFTER: texto y controles legibles, etiqueta Parcial visible y campo 17:30, sin recortes de la región. Las capturas son regionales; no prueban por sí solas el shell completo ni aceptación humana. Axe y overflow sí se ejecutaron sobre la página completa en 1440×1000 y 390×844.

## Reinicio limitado y recursos

Antes de construir se verificaron UID, ejecutable, argumentos/cwd y puertos del harness y su hijo Next; se detuvieron sólo esos procesos. Se reiniciaron con el mismo TLS privado y entorno mínimo, sin credenciales de base de datos en Next. La API fue reconstruida y reiniciada por su launcher validado, conservando el perfil `local-space-wizard`, identidad limitada y destino `espacigo_wizard_20261106`. `/health/live` y `/health/ready` respondieron 200.

No se recrearon cuentas, contenedor, volumen, red, credenciales ni base; no se alteraron migraciones aplicadas. El ensayo se retiene para los gates y revisión pendientes, bajo responsabilidad del ejecutor de este corte; no constituye limpieza final ni aceptación del producto. Las creaciones y la edición tienen recibos privados independientes: no se reejecutaron para lograr verde.

## Pendientes que impiden cierre global

1. Revisión independiente integral frontend/backend/scripts, en curso al escribir este registro; resolver hallazgos antes de publicar.
2. No hay respuesta HTTP anónima positiva respaldada por una fila legalmente elegible del ensayo. La proyección SQL se ejercitó directamente sobre el borrador propio sin cambiar su publicación, y `Detail` lo excluye. El perfil actual no habilita la ruta pública del prototipo; no se utilizará otro destino ni se ampliarán guards o estados para fabricar el gate. Se necesita una vía legal autorizada o una decisión explícita sobre este límite.
3. No hay commit/push de este conjunto, integración a main, build cloud, imagen, revisión/tráfico ni aceptación online. Deben seguir sus gates y autorizaciones separados; las comprobaciones visibles online siguen reservadas al usuario.
