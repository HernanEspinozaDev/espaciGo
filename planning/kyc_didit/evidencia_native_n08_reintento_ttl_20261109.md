# N08-M — reintento deliberado después de TTL (2026-11-09)

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty, sin commit/publicación. Destino exclusivo nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID16388, PostgreSQL180006, ledger91. Sin nueva cuenta ni datos/proveedor externos.

## Criterio y ejecución

C08 canónico no solo exige vencer el intento: debe permitir un nuevo intento deliberado. El probe `kyc-native-expiry.mjs` conserva su modo previo y añade `KYC_NATIVE_EXPIRY_RETRY=1`. El harness exige `NATIVE_KYC_HTTP_EXPIRY_RETRY=1` junto al modo expiry; sigue usando una sola declaración journaled. El decorator cambia únicamente el argumento de reloj del primer intento a CreatedAt+TTL; no modifica reloj host ni realiza SQL ad hoc para sus estados.

`run_kyc_n07_bff.py expiry-retry` terminó probe0/harness0 (10.48s; paquete11.554s), finalizador0:

- Login200, guardar201/readback200; cero creates al guardar, uno al iniciar con consentimiento separado.
- Primer retorno200: EXPIRED/NOT_VERIFIED durable, sin polling adicional; nuevo ingreso al retorno200 conserva expiración.
- Click real Revisar o reintentar lleva a la declaración propia exacta (GET200), sin crear por navegación.
- Consentimiento y segundo click explícitos generan HTTP201 con UUID de intento **distinto**, sin guardar de nuevo declaración.
- GET del original200 confirma EXPIRED/NOT_VERIFIED; GET del nuevo200 confirma PENDING_USER_INPUT/NOT_VERIFIED. Counter realfake: dos creates, no más. Cuatro lecturas de estado totales, tres del original.
- SQL READ ONLY antes de cleanup: exactamente dos intentos propios ligados a la declaración confiable, uno EXPIRED a created_at+30minutes y otro PENDING_USER_INPUT con sesión; una revisión/claim, cero eventos/outbox propios; clockoverride1.
- Storage/URL sin marcadores sintéticos de identidad; exterior0; logout204/sesión401/cookies retiradas.

Backup previo privado `pre-n08-expiry-retry-20261109.dump`, SHA-256 `d0167d8c7d333fa19754d791412595b902106aac1b3c6c833be4c460e693cf63`; custom format, listado/parse completo pg_restore exit0. Prepare/finalize exactos de cleanup ejecutados directamente en terminal Hermes sin leer/imprimir credenciales. Los permisos DELETE temporales son de tabla; la restricción de filas corresponde al helper/journal, no RLS.

## Límites

No transcurso real de30min, cambio del reloj host, SLA ni proveedor externo. Los dos intentos/sesiones fueron sintéticos en el proceso fake propio. Postflight posterior completado: preflight de ambas cuentas sin artefactos PASS; grants audit/outbox DELETE=false y ownerCREATE=false; outbox50/0 preservada; journals/control retirados tras metadata validada. Proceso fake propio retirado al cierre con Next/proxy y puertos de ensayo sin listeners. Este gate no declara por sí solo N08 completo; cierre consolidado en `cierre_native_n08_20261109.md`.
