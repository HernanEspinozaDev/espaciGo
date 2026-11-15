# VI-01 — retiro activo de proveedor en UI, BFF, Go y OpenAPI — 2026-11-10

## Alcance ejecutado

En el worktree existente `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, se avanzó el subcorte VI-01 preservando los demás cambios locales:

- `/mi-cuenta/verificaciones` conserva el formulario/guardado de declaración, pero ya no presenta la acción de inicio biométrico externo. Comunica que la revisión documental propia aún no está habilitada y que no se presenta aprobación prematuramente.
- La tarjeta de Mi cuenta deja de consultar/presentar el panel de estado externo y muestra un estado neutral con enlace a la declaración.
- `/verificaciones/retorno` ya no consulta ni presenta estados externos; limpia los parámetros heredados del URL y orienta a Mi cuenta sin fingir un resultado.
- Los E2E controlados de cuenta, pantalla de identidad y callback comprueban el texto neutral, ausencia de CTA/navegación y cero solicitudes al transporte externo en esas superficies; las pruebas ya no simulan aprobaciones/estados del proveedor y el guard global rechaza tráfico fuera de loopback.
- Se retiraron del adapter TypeScript las operaciones/tipos de creación y lectura de intents y estado del proveedor, además del panel de estado ya desconectado de la UI. Las pruebas de contrato frontend ahora fijan la ausencia de esas exportaciones y conservan regresiones de declaración, RUT y errores de conflicto sin filtrar datos.
- Se retiraron las pruebas del cliente/presentación frontend que dependían del panel externo. El nuevo contrato OpenAPI ejecutable verifica que rutas, operaciones y esquemas del proveedor ya no se publican.
- El gateway BFF rechaza intents, refresh y ambos paths de webhook con `404 route_not_allowed` antes de hacer fetch upstream. Una prueba ejecutable verifica cero llamadas upstream y Playwright consulta cinco rutas contra el Next BFF real; todas responden `404 route_not_allowed`. El inventario y allowlist generados se regeneraron desde el OpenAPI actualizado.
- Los mounts de `cmd/api` usan `NewHandlerWithDeclarationWithoutExternalProvider`; además, todos los constructores del handler devuelven `404` para intent/status/refresh y ambos webhooks. El constructor compatible ignora su argumento legado de servicio del proveedor. Se eliminaron los handlers HTTP Didit y sus pruebas dependientes; la regresión prueba los tres constructores, métodos `OPTIONS` y paths de webhook. El perfil `local-kyc-didit-sandbox-manual` dejó de ser seleccionable, se quitó su constante runtime y el mux local ya no recibe el perfil; no activa startup, workers, rutas de onboarding ni SMTP. Las pruebas de preflight/estado verifican rechazo local sin DB ni API externa. La fábrica de servicio se movió fuera del código productivo a `cmd/api/identity_didit_runtime_test.go` como `newTestDiditService`; el harness nativo sigue siendo opt-in y no se ejecutó.
- Se retiraron las operaciones y esquemas del proveedor de `planning/openapi.yaml`; `api-generate`, generación de inventario/allowlist y `api-check` terminaron correctamente (199 operaciones en el inventario regenerado).
- Se actualizó el plan para reflejar que VI-01 también avanzó en el bloqueo BFF y en el retiro de rutas Go/OpenAPI; el núcleo propio sigue pendiente.

## Verificación local

- `node_modules/.bin/next build` — PASS.
- `node --test tooling/*.test.mjs` — 491 tests: 490 PASS, 0 FAIL, 1 skipped.
- E2E Playwright Chromium local — 6 casos enfocados PASS: estado neutro de Mi cuenta, pantalla sin CTA externo, retorno heredado sin consulta, guardado de declaración, aislamiento A→B/retorno y cinco paths bloqueados por la ruta Next BFF real. Las rutas de API en los recorridos de UI son simuladas; sin DB ni datos reales.
- `node_modules/.bin/eslint` en los archivos de frontend tocados — PASS sin advertencias.
- `node_modules/.bin/tsc --noEmit` — PASS.
- Prettier en los archivos de frontend tocados — PASS.
- `git diff --check` — PASS.
- `go test ./cmd/api -run 'TestLoadRuntimeConfigAllowsPrivatePassfileForLocalKYCProfile|TestRuntimeProfilePinsLocalKYCToNativeTargetWithoutExternalProvider|TestLocalKYCProfileUsesDedicatedStartup|TestLocalKYCProfileNeverLaunchesWorkers|TestLocalKYCMuxExposesOnlyAuthAndIdentityVerificationRoutes'` — PASS; pruebas unitarias seleccionadas, sin conexión a base de datos.
- `go test ./internal/verification/transport/http -count=1` — PASS; incluye la regresión de `404` en los paths retirados para los tres constructores.
- `go test ./cmd/api -run '^$' -count=1` — PASS; compila las pruebas del paquete sin ejecutarlas ni conectar a PostgreSQL.
- `go test ./cmd/api -run 'TestRuntimeProfilePinsLocalKYCToNativeTargetWithoutExternalProvider|TestLocalKYCProfileUsesDedicatedStartup|TestLocalKYCProfileNeverLaunchesWorkers|TestLocalKYCMuxExposesOnlyAuthAndIdentityVerificationRoutes' -count=1` — PASS; pruebas locales seleccionadas, sin conexión a base de datos.
- `go test ./cmd/api -run 'TestRetiredManualDiditRuntimeProfileIsRejected|TestRetiredManualDiditProfileCannotStartRuntimeOrWorkers|TestManualKYCOnboardingRoutesRemainUnavailable|TestManualSandboxRuntimePreflightRejectsRetiredProfile|TestManualSandboxStatusInspectionCannotStartAfterRetirement|TestRetiredManualDiditProfileCannotEnableLocalMailer' -count=1` — PASS; valida retiro del perfil sin DB ni proveedor externo.
- `go test ./cmd/api -run 'TestRetiredManualDiditRuntimeProfileIsRejected|TestRetiredManualDiditProfileCannotStartRuntimeOrWorkers|TestManualKYCOnboardingRoutesRemainUnavailable|TestManualSandboxRuntimePreflightRejectsRetiredProfile|TestManualSandboxStatusInspectionCannotStartAfterRetirement|TestRetiredManualDiditProfileCannotEnableLocalMailer|TestLegacyDiditTestFactoryWiresProvidedRutClaimStore|TestLegacyDiditTestFactoryWiresProvidedSessionDeletionQueue|TestLegacyDiditTestFactoryStaysDisabledWithoutBothDurableStores|TestLegacyDiditTestFactoryUsesDeclarationRutSnapshotWithoutReload|TestLegacyDiditTestFactoryValidatesCallbackBeforeConstruction|TestLegacyDiditTestFactoryRejectsMalformedCallbackOrigins|TestLegacyDiditTestFactoryAcceptsConfiguredCallbackOrigin|TestLegacyDiditTestFactoryRejectsCallbackOriginMismatch' -count=1` — PASS; incluye configuración del helper test-only, sin ejecutar el harness HTTP nativo opt-in.
- `go build -o /dev/null ./cmd/api` — PASS; compilación de producción sin incluir la fábrica de proveedor, que ahora existe solo en `_test.go`.
- Playwright E2E se ejecutó contra el Next standalone local en `127.0.0.1:4203`; seis casos enfocados pasaron. El BFF real se ejercitó directamente para las cinco rutas retiradas; los recorridos visibles interceptaron las rutas de API y no accedieron a DB. El servidor temporal se detuvo y el puerto quedó libre.
- No se ejecutaron migraciones, lecturas/escrituras de DB, proveedor externo, operaciones cloud ni publicación.

## Límites / trabajo pendiente

Esto cubre el retiro de superficies de UI/cliente/BFF y el cierre unitario de rutas en Go/OpenAPI; no cierra VI-01 ni demuestra el ciclo propio. La prueba BFF usa fetch controlado y los E2E interceptan las rutas del API; las pruebas Go son unitarias. No hay evidencia BFF→Go con servicio integrado ni DB nativa. Se conservaron implementación/fixtures de dominio del proveedor; no se borraron datos, secretos, migraciones o recursos residuales. No se implementaron captura propia, adjuntos, flujo de revisión humana, autorización, decisión, almacenamiento GCS/CMEK o limpieza. No se ejecutaron migraciones, pruebas con base nativa, configuración cloud, publicación, integración a `main` ni despliegue.

## Estado del checkout

HEAD al verificar: `274f37e483a078890e92382a159fa49fd23c2fe8`. Los cambios preexistentes del worktree continúan sin confirmar y no se atribuyen a este subcorte. No se hicieron commits ni pushes.
