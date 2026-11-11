# Recuperación tras creación parcial del borrador

- Fecha: 2026-11-10.
- Plan rector: `planning/frontend_next/plan_cierre_flujo_creacion_espacios_20261110.md` (worktree documental `codex/docs/chore/espacios-creation-flow-plan`).
- Rama de implementación: `codex/frontend/fix/space-01-create-postcommit-recovery`.
- Base revisada: `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba` (`origin/main` al iniciar este corte). El primer cambio se publicó como `a5aea39bac4341b415a642297e3c4d375bacd27b`; el candidato del seguimiento partía de `f97518f928159d0c52f49727e1ec39084b71dd27`.

## Brecha y cambio

El wizard ejecutaba `POST /spaces`, luego configuraba zona/horario. Si esa configuración posterior fallaba, mostraba solo un error genérico aunque el borrador ya existía; el botón permitía volver a emitir `POST /spaces`. El alta de espacios no ofrece `Idempotency-Key` en el contrato vigente, así que no se añadió un encabezado no soportado.

Ahora, cuando el POST devuelve un ID y falla una operación posterior, el wizard conserva el ID solo en el estado de esa instancia, informa que el borrador fue creado, mantiene una acción de recuperación visible al recorrer pasos y bloquea una segunda creación. Para sobrevivir al remount de `PrivateSessionBoundary`, conserva fuera del componente únicamente una marca booleana en memoria: la instancia nueva presenta un aviso genérico y enlaza a «Mis espacios», sin trasladar el ID ni exponer el editor del ámbito anterior. Un coordinador fuera de esa frontera limpia la marca al abandonar `/anfitrion/espacios/nuevo`, al completar el flujo o al activar la recuperación; no es persistente tras recargar.

## Evidencia ejecutada

- RED controlada: `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-improvements.spec.ts --grep 'recupera el borrador cuando falla weekly-hours' --reporter=list` reprodujo el error genérico después de confirmar exactamente un `POST /spaces`, un `PUT /availability` y un `PUT /weekly-hours` respondido con 503.
- GREEN original: `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-improvements.spec.ts --reporter=list` — 2 pruebas aprobadas, incluida recuperación y creación exitosa; verifica CTA entre pasos, editor y un único POST.
- RED de cambio de ámbito: la prueba controlada cambió la cuenta devuelta por `GET /auth/session` tras confirmar el POST; `PrivateSessionBoundary` remontó el wizard y eliminó su estado local, por lo que faltaban alerta y recuperación segura.
- GREEN combinado: `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-improvements.spec.ts tests/wizard-partial-account-scope.spec.ts --reporter=list` — 3/3 aprobadas. La regresión verifica aviso genérico tras el cambio, ausencia del enlace al editor, bloqueo del segundo POST, y que salir por navegación de cuenta y volver al wizard limpia el aviso pendiente.
- Revisión de remount en la misma ruta (2026-11-10; fuente basada en `5c0313f66190efe15ea6906382970e07877d4403`): se hizo explícita la aserción de que la ruta permanece en `/anfitrion/espacios/nuevo` tras la revalidación que devuelve otro `account_id`, y que el aviso genérico sigue visible después del remount de `PrivateSessionBoundary`. `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-partial-account-scope.spec.ts --reporter=list` — 1/1 aprobada.
- Gates globales ejecutados sobre el candidato: `format:check`, `typecheck` y `build` pasaron; el build Next.js 16.4.3 generó 32 páginas estáticas. `lint` terminó sin errores y con 4 warnings preexistentes en `listing-gallery.tsx` y tooling. `test`: 480 aprobadas, 1 skip opt-in por falta de `BASELINE_DRAFT_ADAPTER_SOURCE`, 0 fallidas.
- `git diff --check` pasó.

## Límites y siguiente gate

Esta evidencia es de UI controlada con respuestas sintéticas; **no** prueba persistencia Go/PostgreSQL, API real, almacenamiento, release ni flujo online. No se creó ninguna cuenta o espacio real, no se consultó ni escribió base de datos, y no se modificaron OpenAPI, Go, SQL, migraciones KYC ni versiones de base de datos.

La limitación de contrato permanece: si el propio `POST /spaces` se confirma en servidor pero se pierde su respuesta antes de que el cliente reciba el ID, esta UI no puede reconciliar con certeza ni garantizar que un reintento manual no duplique. Requiere resolver el contrato/reconciliación en una tarea separada, sin inventar soporte de idempotencia. La marca de recuperación entre remounts solo existe en memoria del runtime de la pestaña y se limpia al dejar la ruta de creación; la prueba no afirma recuperación tras recarga. Todos los E2E de este corte son controlados con respuestas sintéticas: no prueban persistencia Go/PostgreSQL, API real, almacenamiento, release ni flujo online. La fase de creación end-to-end sin mocks sigue parcial y requiere la siguiente brecha del plan con un target sintético/local expresamente autorizado.
