# Bitácora — entrega del avance de arriendo a main

**Fecha:** 2026-11-15.
**Encargo:** publicar todo el avance existente del plan de arriendo, incluida la demo, su preparación comercial y las evidencias; no declarar el producto comercial terminado ni habilitar migraciones pendientes.

## Fuentes y preservación

- Base de integración: `7b01a47370d119a0433190c25a9fad7f55dc3dc6`, confirmada con `git fetch origin --prune`.
- Candidato combinado: `/home/nandev/Documentos/projects/espaciGo-rental-main-delivery`, rama `codex/marketplace/chore/rental-02-main-delivery`.
- Fuente comercial preparada: `/home/nandev/Documentos/projects/espaciGo-rental-history-candidate`, HEAD `aa53379db0da0e3b872ce23e8258becb324a2b4e`. Se fijó su árbol completo en `2bd6efd60851eb31ce9274a2ae3d20d561823d3f`: 157 rutas dirty inventariadas; importación mediante índice temporal sin cambiar su índice, archivos ni HEAD.
- Fuente DEMO-05 más reciente: `/home/nandev/Documentos/projects/espaciGo-rent-published-space-monthly-calendar`, HEAD `7b01a47370d119a0433190c25a9fad7f55dc3dc6`. Se reconciliaron sus once rutas de trabajo sobre el candidato comercial. Los otros worktrees, cuentas, datos y demos retenidas se preservan.
- No se importó el worktree KYC completo: se conserva únicamente el candidato compartido ya preparado y su historia SQL revisada. No se arrastra la V85 comercial antigua del worktree original.

## Qué incluye esta entrega

1. **Disponibilidad/cotización/preview comercial preparados:** puertos/adaptadores tipados, importes int64 exactos, retorno seguro tras acceso, revalidación del intervalo e invalidación por cambio de sesión. La cotización no retiene horario y el preview no firma ni paga.
2. **Persistencia y concurrencia locales:** snapshots de quote, expiración, locks de cuenta compartidos con escritores de publicación/supresión/autenticación y lector mínimo de identidad cifrada ligado a la revisión actual. Pruebas PostgreSQL descartables; no equivalen a integración comercial del navegador ni a autoridad cloud.
3. **Historia única V1–V97:** V1–V96 preservadas; el SQL comercial nunca aplicado queda en V97. V85–V97 continúan `review_required_migrations`, `automatic=false`. No se habilita ninguna de ellas.
4. **Preparación de release:** contrato Go/JSON/Python de autoridad, limpieza de Jobs propios y modo explícito de preflight-only; la ruta normal mantiene la política exacta previa a backup/apply/rollout.
5. **Storage preparado y probado con fakes:** lectura GCS ligada a generación/metageneración y documentos cifrados por propósito/keyring. No demuestra permisos, custodia o durabilidad reales en Cloud Run.
6. **Demo mensual separada:** Go en memoria + Next/BFF, aprobación/pago/confirmación simulados, ambos calendarios coordinados, accesibilidad y fencing de respuestas tardías. `sessionBusy` impide login/logout concurrentes aunque cambie el contexto. El readback de precio lee el HTTP 200 de la solicitud GET capturada al volver al espacio editado; no es persistencia durable.
7. **Reconciliación final:** se mantienen ambos comandos E2E (comercial y demo), el guard de contexto sin cambios efectivos, y la corrección portable del daemon. El generador de inventario ahora reconoce el guard Go real `if localBookingTrialEnabled {`; se regeneró el inventario, no se editaron los artefactos a mano.

## Pruebas ejecutadas en el candidato combinado

| Gate | Resultado |
| --- | --- |
| `pnpm install --frozen-lockfile --offline` | PASS; sin cambiar lockfile. |
| `pnpm api:generate` y `pnpm api:check` | PASS; 216 operaciones. El primer generate detectó el marcador Go antiguo; se corrigió el generador y se repitió. |
| `pnpm test` | PASS; 568 tests, 567 pass, 1 skipped, 0 fail. |
| `pnpm typecheck`, `pnpm lint`, `pnpm format:check` | PASS; lint 0 errores y 12 advertencias. |
| `pnpm build` con entorno limpio | PASS; artefacto standalone del candidato combinado. |
| Playwright `playwright.rental-demo.config.ts` | Dos ejecuciones consecutivas PASS, 2/2 cada una; Go en memoria + Next/BFF loopback. |
| Playwright `playwright.public-rental-commercial.config.ts` | PASS, 1/1; UI con fixtures/intercepción, no cotización comercial persistida E2E. |
| `env -i ... go test ./... -count=1` | PASS; las integraciones opt-in se ejecutan aparte en los harness. |
| `env -i ... go vet ./...` | PASS. |
| `python3 -m unittest discover -s scripts/gcp -p 'test_*.py'` | PASS, 200 tests; proveedor sintético, no despliegue real. |
| `scripts/test-rental-combined-postgres.sh` | PASS, 26 checkpoints requeridos, ninguno omitido; `-race`, PostgreSQL descartable network-none. |
| `scripts/test-migration-runner-postgres.sh` | PASS, 93 resultados DB requeridos, ninguno omitido; V1–V97/fresh/upgrades/autoridad/quotes y disponibilidad. |

Ambos contenedores de ensayo y sus directorios propios se retiraron con verificación por ID exacto. Los puertos 4205/18117 y 4208/18089 quedaron libres tras los E2E. No se ejecutó la suite Playwright general ni se presentan sus fallos históricos como resueltos. Los logs locales de este corte están bajo `/home/nandev/.hermes/cache/scratch/rental-*`.

## Publicación y límite del pipeline

La instrucción expresa de publicar este avance a main separa **entrega de código** de **cierre del plan y habilitación cloud**. El trigger activo `548b8eff-e489-4307-a77f-455b56aa4e57` observa `^main$` y usa `cloudbuild.yaml`. Se leyó el watermark real: fuente `e789d6eb9431c643f9c7e6ec0f9508e431ca7b2e`, build `0e22852b-7fde-42a5-a5b0-0160f151275a`, esquema registrado 84; la entrega se clasifica `full`, no web-only. Ese recibo no sustituye un ledger actual.

La política de esta entrega **rechaza V85–V97 pendientes antes de backup/aplicación/rollout**. Se ejecutó además `release_main.migration_policy` real contra cada uno de los trece nombres/checksums exactos: 13/13 rechazados, sin llamadas al proveedor. El plan privado puede fallar antes por autoridad; ese fallo no se evita cambiando permisos ni SQL. No se cambia el trigger, no se omiten checks, no se habilita el flag comercial ni se ejecuta una migración manual para obtener verde. La publicación puede quedar en main aunque el release automático quede detenido en sus gates; Git publicado no equivale a aplicación SQL o despliegue exitoso.

El SHA publicado, igualdad con `refs/heads/main` y estado remoto de CI/build se verifican después del push y se entregan en el recibo de la conversación; este documento no inventa anticipadamente esos resultados ni un SHA autorreferencial.

### Revisión final y adjudicación

- `deleg_d3f0e606`: readback DEMO-05 reforzado aprobado, sin bloqueadores concretos.
- `deleg_9030ebf0`, hoja semántica: integración combinada aprobada; guard de contexto, fencing/sesión, marcador de inventario y ambos comandos E2E preservados. Revisión de fuente, sin ejecución por el revisor.
- La hoja de seguridad de release confirmó que la policy rechaza V85–V97 antes de backup/apply/deploy y que el Job de plan utiliza transacción READ ONLY sin ejecutar SQL de migración ni crear ledger. Emitió `passed=false` porque el release normal no podrá terminar si esas versiones siguen pendientes. Se acepta ese límite de **release**, pero no se interpreta como impedimento de **publicar el avance solicitado**: el rechazo cerrado es precisamente el control que mantiene deshabilitada su aplicación. No se modifica la policy, el trigger ni sus permisos para lograr un deploy. A06 y la habilitación comercial permanecen abiertos; un futuro release exige su corte operativo completo. No se afirma ledger live ni autoridad cloud a partir de esta revisión.

## Qué falta para completar el plan

- **A05/A06:** ledger, autoridad y catálogo cloud actuales ligados al SHA revisado, preflight privado y decisión específica por migración; backups/gates antes de cualquier aplicación. No se sustituye por un harness local.
- **A09:** perfil/autenticación comercial aptos para cloud y clasificación/custodia documental durable; los flags comerciales siguen exclusivamente locales y cerrados por defecto.
- **A10/A11:** recorrido comercial compuesto navegador→BFF→Go→PostgreSQL con identidad sintética autorizada y readback independiente. La persistencia de quote está probada en DB descartable; el E2E comercial actual usa fixtures, por lo que no cierra esta integración.
- **Producto:** revisión humana/QA online reservada al usuario. No hay aceptación inferida, firma, pago ni reserva comerciales reales.
- Las tablas y evidencias históricas mantienen sus fechas y límites; la publicación de este avance no las convierte en capacidades productivas terminadas.
