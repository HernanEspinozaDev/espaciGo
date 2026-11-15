# N08-A — inputs inválidos sin mutaciones (2026-11-09)

## Alcance y fuentes

- Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado.
- Criterios parciales C01/C02 de `matriz_aceptacion_native_20261109.md`. No cerrar N08 ni C11 por medir dos viewports.
- Probe versionado nuevo: `frontend/tooling/kyc-native-validation.mjs`, SHA-256 `083e267d58120df1b07fde8bc46299097c7be89cd1802c6088c8b2c4b36d7274`.
- Browser aislado, HTTPS4219→Next standalone4201→API harness18090→PostgreSQL nativo `espacigo_dms_stage_20261020`, OID16388, motor180006. Mismo build N07 `espacigo-kyc-n07-fixed-k653aibr`, sin cambios de producto para este slice.
- Cuenta sintética existente, política y receipt privados validados mediante preflight Go antes del login; ninguna cuenta creada/reset. El probe lee solo el receipt autorizado en memoria, con validación de owner/tipo/modo0600 y parent0700; cero valores privados en reportes.

## Revisión parental del candidato

La entrega source-only `deleg_f3c82d00` creó el archivo, sin ejecución nativa. Padre corrigió antes del ensayo tres rutas inventadas `/api/auth/*` por las existentes `/api/backend/auth/*`, añadió comprobación explícita login200, owner de fixture/parent, espera de alerta real y assert de formulario visible. También dejó ausencia de opt-in como exit2, no como ejecución exit0 sin prueba.

`node --check`, Prettier y ESLint focalizados PASS. Subprocess offline sin opt-in y con argumento inesperado rechazados con exit2 y mensaje fijo antes de leer archivos privados. No se cambió gateway, OpenAPI, formulario, reglas de edad, consentimiento ni seguridad para obtener verde.

## Preparación y ejecución real

1. Preflight nativo READ ONLY PASS, antes de abrir ventana de permisos.
2. Backup privado nuevo `.local/testing/kyc-native/pre-n08-validation-20261109.dump`, 0600, 8,321,683bytes, SHA-256 `5102c82fcd23b3c5aa19804a7f0675d4c6330482407d497277dc09ac30cb336b`; `pg_restore --list` y consumo completo `pg_restore --exit-on-error --file=/dev/null` PASS. No restore.
3. Prepare guardado `db/repair/prepare_native_kyc_n07_http_cleanup.sql`, ejecución parental directa como administrador local. El harness exige autoridad de cleanup antes de servir aunque este probe no llegue a guardar. DELETE temporal de tabla, no RLS; runtime no elevado.
4. Orquestador `run_kyc_n07_bff.py validation` en scratch ejecutó el probe versionado con `KYC_NATIVE_VALIDATION=1`, entorno browser allowlist, y el harness con `-race`. La invocación equivalente del probe, con harness/preflight ya preparados, es `KYC_NATIVE_VALIDATION=1 node frontend/tooling/kyc-native-validation.mjs` desde el repositorio. No ejecutar fuera de ese procedimiento.
5. Finalmente, stop marker privado→harness cerrado→finalizador directo `db/repair/finalize_native_kyc_n07_http_cleanup.sql`, incluso ante fallo. No se dependió de `sudo -n` de un proceso background.

## Resultados por caso

| Caso | 1365×900 | 390×844 | Evidencia visible esperada | Intentos de mutación |
| --- | --- | --- | --- | --- |
| Falta nombre obligatorio | PASS | PASS | Invalididad nativa del control; formulario sigue visible | 0 |
| DV incorrecto | PASS | PASS | `aria-invalid=true` y alerta «Revisa el RUN/RUT» | 0 |
| Menor de18 | PASS | PASS | Alerta «Revisa el RUN/RUT» con fecha sintética relativa UTC | 0 |
| Fecha de nacimiento futura | PASS | PASS | Invalididad nativa por límite máximo | 0 |
| Consentimiento de declaración ausente | PASS | PASS | Checkbox requerido inválido | 0 |

Reporte real: **ok=true,10casos**, GET inicial declaración404 en los10casos, mutationAttempts=0, externalDenied=0. El guard habría contado y abortado cualquier POST/PUT/PATCH/DELETE de verificación; éxito exige cero intentos observados, no solo que los abortados no se persistieran. Todos los casos conservaron el formulario y no mostraron resumen de datos guardados.

Login real200 y sesión200 antes de los casos. Logout final204, sesión401 y ambas cookies de sesión ausentes. No se guardó ninguna declaración, creó intento, llamó proveedor, capturó pantalla ni generó trace con datos privados.

**probe_exit=0, harness_exit=0**, `TestNativeKYCAPIWithBFF` PASS7.41s (paquete8.489s con race). Finalizador COMMIT.

## Readback y regresión posterior

- Readback administrativo READ ONLY independiente: audit DELETE=false, outbox DELETE=false, owner CREATE=false, SELECT de columnas de audit=false, ledger90.
- Preflight fixture posterior PASS: ambas cuentas sintéticas existentes activas/autorizadas y sin artefactos de declaración/intent/event/claim/audit propios.
- Journal de declaración ausente, marker ready ausente, stop propio retirado tras validar metadata, API18090 sin listener. No se borraron cuentas, colas ajenas ni recursos de otros ensayos.
- Go sin caché `go test -race -count=1 ./cmd/api ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/kyc-fake-provider`, vet de paquetes afectados y `git diff --check`: PASS.
- Next4201/proxy4219/fakeprovider19091 propios siguen retenidos por el agente operador exclusivamente para el siguiente slice N08, según evidencia N07. No se arrancó worker general.

## Estado y siguiente slice

**N08-A completa para los cinco rechazos descritos en ambos tamaños.** C01/C02 permanecen parcialmente cubiertos, no una validación exhaustiva de todo Chile ni de todos los endpoints. C11/Axe, ciphertext, webhook, decisión y retención no se acreditan por este probe.

Siguiente N08-B: conservar el probe de flujo básico N07 en tooling versionado y añadir retorno propio/ajeno, saneamiento de query antes de GET, ignorar `status=Approved` y retry ante denegación real404. En implementación source-only separada; aún no ejecutada. Sin nuevas cuentas, proveedor externo, datos reales, aceptación legal/humana, publicación, main ni despliegue.
