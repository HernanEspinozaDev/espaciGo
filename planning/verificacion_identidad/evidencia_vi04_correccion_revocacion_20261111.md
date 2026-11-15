# Evidencia — VI-04, acción de corrección y estado revocado

- **Fecha:** 2026-11-11
- **Estado:** subcorte frontend completado con fixtures sintéticos; VI-04 global sigue parcial.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se preservó. Este recibo atribuye solo el subcorte descrito, no cambios ajenos.

## Alcance

- Para `requiere_antecedentes`, Mi cuenta presenta «Corregir documentos» y enlaza a `/mi-cuenta/documentos`.
- El recorrido Playwright selecciona un expediente sintético en ese destino y verifica que aparece el formulario de evidencia editable.
- Subcorte correctivo: un caso sintético `requiere_antecedentes` parte con `identity_back` bloqueado. El mock rechaza la carga si el `If-Match` no es `"7"` o si el multipart no contiene exactamente el rol `identity_back` y el archivo sintético esperado (nombre, MIME y contenido). Tras aceptar el fixture, la UI muestra el reemplazo pendiente y no ofrece reenviar. Un GET mockeado de la lista cambia el fixture a limpio; no se ejecuta ni se prueba un scanner. El mock de envío solo acepta el ETag actual `"8"`; la UI entonces presenta `en_revision`. Todas las rutas API siguen interceptadas.
- Subcorte de recuperación de carga incierta: un upload sintético recibe 409 y el GET explícito confirma `en_revision`; la UI limpia el `File` retenido y el input nativo. Cuando otra lectura fixture posterior vuelve a `requiere_antecedentes`, el input aparece vacío, «Cargar evidencia» sigue deshabilitado y no hay un segundo upload ni envío automático. Solo se demuestra comportamiento de UI con estados mockeados.
- Complemento de recuperación: en un escenario separado, el 409 seguido de GET que confirma que el caso sigue en `borrador` conserva el archivo local. No hay una segunda carga automática; la carga ocurre únicamente tras un clic explícito y usa el ETag leído. El mock registra cero envíos.
- Para `revocada`, la tarjeta conserva el texto neutral de revisión revocada y enlace al estado de solicitud. Un fixture con `verified_until` futuro no produce «Cuenta verificada» ni «vigente hasta» en cliente.
- La respuesta sintética de detalle conserva un ETag fuerte para probar la carga versionada del caso; los tests siguen interceptando la API y no comprueban BFF→Go.

## TDD y verificaciones

- **RED:** `Mi cuenta ofrece corregir documentos cuando la revisión requiere antecedentes` falló antes del cambio: no encontró el enlace «Corregir documentos».
- **GREEN:** tras derivar el rótulo del CTA de `identityState`, el escenario de corrección y el de revocación pasaron, **2/2**.
- El test browser A→B `Mi cuenta reemplaza el resumen de A tras un cambio de sesión a B` pasó en su primera ejecución: publica `session_changed`, confirma sesión B, espera una nueva lectura y verifica que el resumen de A desaparece. No se cambió producción en este subcorte porque la clave de sesión y el descarte de datos de la fachada ya existían.
- El test browser `Mi cuenta conserva el resumen de B si la respuesta pendiente de A se libera después del cambio` captura el titular A cuando el mock recibe la lectura retenida y lo afirma antes del cambio; cambia la sesión sintética a B, espera que el resumen de B esté visible y libera después la respuesta de A con HTTP 200. B permanece visible; no reaparecen aprobación de A ni error de consulta. No se cambió código de producción.
- El test browser `Mi cuenta vuelve a cargar el resumen propio tras un ciclo A→B→A` verifica las dos respuestas de sesión, una nueva lectura en cada transición, estado de corrección mientras la sesión es B y regreso al resumen aprobado de A al volver a A. No se cambió código de producción.
- El test browser `Mi cuenta permite reponer evidencia bloqueada y solo reenviar tras validación sintética` pasó **1/1**. El mock hace fail-closed ante multipart o ETag incorrectos; el flujo verifica la solicitud correcta del rol/archivo, bloqueo mientras el fixture está `pendiente`, GET fixture que lo presenta `limpio` y reenvío con el ETag siguiente. No requirió cambios de producción.
- **RED:** `clears a selected identity file after refresh makes the case non-editable` falló como esperado: tras 409 y refresh de `en_revision` a `requiere_antecedentes`, «Cargar evidencia» quedó habilitado aunque el input recién montado estaba vacío.
- **GREEN:** `handleRefresh` descarta el `File` y limpia el input cuando el caso autoritativo recién leído no es editable. El retorno posterior a `requiere_antecedentes` deja el formulario sin archivo seleccionado; el mock registra una sola carga y ningún envío.
- El primer intento del escenario demorado capturó una respuesta de sesión A iniciada por el evento de foco; el matcher se afinó para esperar el `account_id` B y la repetición pasó. El mock entrega la respuesta HTTP después del cambio; esto no fuerza a que `fetch` aplique un resultado tras una señal de aborto.
- Una revisión delegada read-only señaló que el test no ligaba explícitamente la lectura retenida con A. Se corrigió capturando y afirmando `activeAccountId === accountId` al entrar la petición al mock, y generando la respuesta tardía desde ese titular; el test focal pasó **1/1**. El veredicto final de esa revisión fue internamente contradictorio y no aportó una evaluación utilizable; no se cuenta como revisión independiente aprobatoria.
- `./node_modules/.bin/next build` — PASS después del cambio de producción para limpiar el archivo local; Playwright usó el standalone recién construido.
- Prettier focalizado — PASS; ESLint focalizado — PASS; `tsc --noEmit` — PASS.
- `node --test tooling/document-verification-facade.test.mjs` — **11/11 PASS**, incluidos descarte de respuesta tardía al cambiar cuenta y abortos.
- Playwright local Chromium, standalone en `127.0.0.1:4203`:
  - `tests/didit-account-status.spec.ts` — **14/14 PASS**.
  - `tests/identity-upload-recovery.spec.ts` — **2/2 PASS**, incluyendo limpieza al confirmar caso no editable y conservación del `File` mientras permanece editable.
  - `tests/phase6-account.spec.ts -g '/mi-cuenta: 404 inicial|@a11y account routes'` — **2/2 PASS**, incluida accesibilidad Axe.
  - `tests/phase-f6-document-verification.spec.ts` — **3/3 PASS**.
- La revisión independiente `deleg_9148374c` no reportó un defecto en el recorrido correctivo de VI-04; su hallazgo concreto fue sobre IDs repetidos en el fixture duplicado VI-02 y quedó corregido/documentado en `evidencia_vi02_carga_roles_20261111.md`.

## Límites y gates pendientes

- `DocumentVerificationOwnerCase` no expone un motivo de decisión seguro para el titular; no se inventó ni se mostró un motivo. El contrato y backend aún requieren definir/entregarlo para satisfacer ese criterio.
- El recorrido correctivo demuestra que la UI envía una carga sintética al mock, conserva el rol y ETag, y condiciona el reenvío al estado devuelto por una lectura mockeada. No demuestra aceptación o inspección real del archivo, CAS/almacenamiento, transición durable ni persistencia tras recarga/reinicio.
- La revocación se representa con un fixture, no demuestra decisión ni persistencia. Los recorridos A→B→A y la respuesta A retenida se prueban con sesiones/casos sintéticos y API interceptada; no demuestran autenticación real ni una respuesta tardía atravesando BFF→Go después de cancelar el request. El puerto que ignora aborto sigue probado en la fachada/unidad.
- Los tests interceptan `/api/backend/**`; no demuestran autenticación real, BFF→Go, ownership/autorización en servidor, PostgreSQL, almacenamiento, recarga/reinicio ni cálculo de vigencia autoritativo.
- **No** se ejecutaron SQL/DDL ni migraciones; no se conectó a ninguna base ni a Cloud; tampoco hubo publicación, commit, push, integración a `main` o despliegue.
- VI-04 permanece parcial. Este subcorte no es aceptación de producto ni autorización para usar documentos/datos reales.
