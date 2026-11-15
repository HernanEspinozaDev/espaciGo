# Publicación del avance KYC a main — 2026-11-15

## Autorización y alcance

El usuario pidió expresamente subir todo el avance del plan KYC a `main`, aunque no funcione en Google Cloud. Se entrega trabajo en progreso: no se declara completo el MVP, la persistencia/reinicio ni su operación cloud.

Fuentes conservadas:

- Backend/SQL/tests/plan desde `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, base `274f37e483a078890e92382a159fa49fd23c2fe8` más cambios locales.
- Avance KYC frontend y sus pruebas/evidencias desde el worktree `espaciGo-frontend-next-hermes`, sin limpiar ni escribir en el checkout original.
- `origin/main` leído tras fetch: `a3d57b3357fe05da9f1eb7743c6cbd401d62cc9a`, con avances posteriores de arriendos/espacios que deben preservarse.
- Integración aislada en `espaciGo-kyc-main-publication`, rama `codex/marketplace/chore/kyc-07-main-publication`; snapshot inicial `18bfc90`.

La selección inicial incluyó 862 paths de código, pruebas, SQL, scripts y documentación vinculados al trabajo acumulado. Se excluyeron los ejecutables locales `api` y `bin/dbmigrate`; no se copiaron secretos, `.local/`, dependencias ni datos persistentes. El frontend añade después los archivos KYC que solo existían en su worktree.

## Revisión de publicación y límites

Tres revisiones iniciales de solo lectura identificaron el riesgo de publicar indiscriminadamente el worktree, la necesidad de reconciliar main y los archivos frontend faltantes. No constituyeron una revisión exhaustiva de cada línea ni una aprobación operacional. Se preservan las herramientas de ensayo explícitamente loopback (`cmd/kyc-fake-provider` y `cmd/local-kyc-mailbox`), sus tests y el historial SQL/operativo versionado como parte reproducible del plan; no se incluyen en el Dockerfile runtime ni se ejecutan por esta publicación. Los scripts `db/repair` son fuentes históricas/operativas, no autorización para ejecutarlos.

El escaneo local de patrones de claves privadas, tokens GitHub, claves Google/AWS y URLs con contraseña no confirmó secretos reales en el conjunto inicial. Las cuatro coincidencias de URLs correspondían a fixtures/ejemplos con contraseña `***`. Es una comprobación acotada, no garantía de ausencia de todo dato sensible.

Los 97 archivos de migraciones existentes en `origin/main` se compararon byte a byte contra el candidato y permanecían idénticos. No se ejecutó SQL ni se concedieron ACL. V85–V96 conservan la política de revisión manual `automatic=false`; el push no sustituye los gates de autoridad, backup, ledger, compatibilidad ni aceptación.

## Verificación local de preparación

- Integridad del snapshot original: sin cambios frente al manifiesto privado de preparación tras copiarlo al worktree aislado.
- `scripts/test_kyc_manual_local.py`: 11 pruebas PASS con entorno mínimo y `LANG=C.UTF-8`. El primer intento omitió LANG y falló una aserción de entorno; se corrigió la invocación, no el código.
- `node --test scripts/test_kyc_manual_https.mjs`: 2 PASS, 0 FAIL, pruebas sintéticas locales.
- PyYAML faltaba en el Python global; se instaló `PyYAML==6.0.3` únicamente en un venv scratch de comprobación, sin cambiar dependencias del repositorio.

## Árbol reconciliado y verificación final local

- Se conservaron ambas historias mediante merge normal de `origin/main`, sin rebase/reset/force-push. Se incorporaron las features, rutas y harness KYC ausentes del checkout backend. La integración no elimina las rutas comerciales o de espacios de main.
- La reconciliación resolvió la colisión de `DeclarationKeyring`: los consumidores de lectura usan `DeclarationOpener`, mientras el servicio KYC conserva `Seal` y `Open`. Los perfiles de espacios y KYC mantienen sus destinos y guardas separados; el CLI rechaza mezclar flags nativos con recovery cloud.
- El harness KYC previo conserva su receta acotada a V96 en `identity_schema_v96_bootstrap_test.go`, con un nombre diferenciado; el harness actual de main conserva V97. El checker CI exige ambos conjuntos. Son pruebas opt-in preparadas; no se ejecutó PostgreSQL en esta publicación.
- OpenAPI, cliente, allowlist BFF y matriz regenerados: **215 operaciones**. Se conservaron las rutas rental/espacios/auth de main y se retiró el contrato público del proveedor abandonado. Se mantuvo el mensaje owner sanitizado y la fecha civil del revisor.
- `go test -race -count=1 -json ./...`: **exit 0; 5039 eventos PASS, 231 SKIP, 0 FAIL**. Los conteos incluyen subtests; los opt-ins omitidos no prueban DB. `go vet ./...` y `go build ./...`: PASS. Tras los últimos ajustes solo de OpenAPI, se reejecutaron con race `internal/openapicontract`, `internal/verification/...`, `cmd/api`, `cmd/dbmigrate` e `internal/migrator`: PASS.
- Python `unittest discover -s scripts/gcp -p 'test_*.py'`: **224 pruebas PASS** con entorno mínimo; usa fakes, no Cloud. Se corrigieron fixtures del suffix de migración para preservar V97 y el diagnóstico compartido de autoridad, sin debilitar validaciones.
- Frontend tooling, repetido tras la corrección de aislamiento local: **639 pruebas; 638 PASS, 1 SKIP, 0 FAIL**. TypeScript, API check y Prettier: PASS. ESLint: exit 0, **0 errores y 13 warnings** (no se desactivaron reglas). `next build` repetido tras la corrección UI: PASS, generación estática **34/34**. Dependencias instaladas offline con lockfile en el worktree aislado, sin `.env` ni configuración privada.
- Playwright local Chromium en `127.0.0.1:4211`, conjunto final repetido completo: **18/18 PASS**, owner completo, corrección, cola/revisión y los dos recorridos A→logout→B→logout→A de declaración y estado documental. APIs y scanner interceptados/sintéticos; no prueba BFF→Go/DB, scanner real ni persistencia. Los primeros intentos detectaron un puerto distinto del guard fijo, expectativas antiguas de Bearer en el navegador y prefetch de páginas legales nuevas. Se usó el puerto fijado, se alineó el fixture con la sesión BFF de main (sin Authorization del navegador, login sintético requerido) y se añadieron únicamente las dos rutas legales a la allowlist de navegación HTML/RSC; el bloqueo de API inesperada y de orígenes externos permanece.
- Escaneo de los 888 paths cambiados frente a main: sin binarios, `.local` ni dependencias añadidos; el escaneo acotado no encontró claves privadas/tokens reales y mantuvo cuatro coincidencias sintéticas de URL ya revisadas. No es una garantía universal de ausencia de secretos. Los 97 SQL de main siguen byte a byte. El snapshot del worktree backend original permanece sin cambios.

### Adjudicación de revisión fuente

- Backend/runtime: revisión independiente sin hallazgos concretos de lógica o seguridad en el diff acotado, sin ejecutar comandos ni DB.
- Frontend: se confirmó que la importación había retirado el reset de estado local por sesión. Se corrigió con una frontera React keyed por estado/cuenta/roles/capacidades/modo/expiry que desmonta selección, mensajes y preview del contexto viejo; se conservaron la cancelación/generación del runtime y se restauraron los catches `AbortError` y el controller del efecto. La nueva prueba de estructura no sustituye comportamiento browser. El primer intento delegado quedó parcial; el padre corrigió el tipo `AccessToken` y la expectativa inválida de prop del panel, y repitió todas las pruebas unitarias, TypeScript, lint, formato y build.
- La observación sobre ordenar expedientes se rechazó con fuente: `ListOwnCases` ya usa `ORDER BY c.updated_at DESC,c.id` y filtra superseded; el cliente mantiene ese orden. No se agregó un ordenamiento incorrecto por `revision` entre IDs distintos.
- La re-revisión independiente acotada de la corrección confirmó ausencia de hallazgos concretos de lógica/seguridad y validó la adjudicación del orden de casos. Fue exclusivamente fuente, sin comandos, pruebas, browser ni DB; no aprueba el árbol completo ni sustituye evidencia conductual integrada.
- Un ensayo ampliado a dos casos históricos adicionales de `phase6-account.spec.ts` dio inicialmente **16 PASS / 2 FAIL**. Se corrigieron sus selectores, ID/RUT/versión de consentimiento y el envelope de error contra el contrato real, fijando la expiración del fixture. El estado documental se consulta en General, como en main, sin duplicar la tarjeta en la declaración. Se mantiene el contador del único 409 y las aserciones de almacenamiento privado, separación A/B y ausencia de proveedor. Un intento posterior detectó una carrera solo en la aserción síncrona de `detailReadbacks`: ahora espera explícitamente que llegue el segundo readback antes de comprobar sus mismos campos. Se conservan estos fallos como historial; el conjunto final repetido completo dio **18 PASS / 0 FAIL**. No se degradó el parser ni se cambió el producto para satisfacer fixtures.

No se realizó comprobación funcional online, reinicio PostgreSQL, restore, acceso a Didit ni ejecución manual de Cloud Build/Jobs. La revisión fuente final y el readback remoto se mantienen separados de estas pruebas locales; el SHA final de publicación se obtiene del commit que incluye este documento, sin inventar un hash autorreferente. Publicación Git no significa despliegue cloud exitoso ni cierre del plan.
