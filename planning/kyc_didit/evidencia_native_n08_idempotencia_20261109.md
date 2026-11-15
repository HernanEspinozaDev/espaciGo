# N08-E — idempotencia medida en proveedor falso (2026-11-09)

## Baseline y alcance

- Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado.
- Probe probado `frontend/tooling/kyc-native-flow.mjs`, SHA-256 `c027cfb21f9ba4ab629007c93824dd04793284a4ec4f885074f3f2c3e48a1f65`.
- Fake source `cmd/kyc-fake-provider/server.go`, SHA-256 `a7b28626812135eba9bcd60ead4d11f9b1b6fb4ab4ba1068b0845377782771a8`; binario del ensayo `kyc-fake-provider-n08e-20261109`, SHA-256 `c716d752d74cbae6f894da2e430b1a0a366e1448d59b694437d40ef597f806e3`.
- Mismo frontend/build N07 y destino nativo exclusivo `espacigo_dms_stage_20261020`/OID16388/motor180006. Una declaración journaled y un intento de las cuentas sintéticas existentes autorizadas; sin nuevas cuentas ni datos reales.

## Observabilidad y revisión

El fake agrega `GET /__local_test/stats`, protegido por la misma clave sintética de su perfil local, rechazo de query y headers no-store/nosniff. Devuelve **solo dos enteros**: `creation_requests` y `active_sessions`. El contador incrementa al entrar a un POST autenticado de creación, incluso si el payload es inválido; no oculta un intento extra fallido. Contadores/mapa usan mutex; no se exponen IDs/vendor_data/PII ni se cambia un endpoint del producto.

Entrega `deleg_44ffd0ff`: RED stats404 antes del handler, GREEN unitario de autenticación/esquema exacto/creación válida e inválida/delete/negativas. Padre leyó ambos archivos y ejecutó `go test -race -count=1 ./cmd/kyc-fake-provider` y vet: PASS. Compiló el binario citado y reemplazó únicamente el proceso fake propio anterior tras validar UID/cwd/exe y liberar19091, con preflight de fixtures previo. Nuevo proceso4081551, entorno mínimo; identidad binaria y stats200 con0creates/0sessions releídos.

Entrega `deleg_7c5479a3` en un único probe: padre revisó las lecturas de métricas, captura en memoria del body/key auténticos y replay. La ausencia de Prettier reportada por cwd del ejecutor no bloqueó: padre ejecutó el binario realmente instalado en `frontend/node_modules/.bin`, formato/sintaxis/ESLint/TypeScript PASS. No se cambió UI/Go de producto ni se dio secreto real a Next/probe.

## Preparación

Preflight nativo READ ONLY PASS; suites Go seguras con `-race -count=1`, vet y diff-check PASS, sin autodiscovery de credenciales ni lifecycle antiguos habilitados.

Backup nuevo `.local/testing/kyc-native/pre-n08-idempotency-20261109.dump`, 0600, 8,322,575bytes, SHA-256 `65fb0802fb02db060b5986dd24447f68d578c9d2f6bbc6969b454cb881bc5b50`; `pg_restore --list` y consumo completo PASS. No restore.

Prepare/finalize SQL versionados por sudo directo parental. `run_kyc_n07_bff.py idempotency` lanza Go con expectativa de un evento/intento y el probe con `KYC_NATIVE_FLOW=1 KYC_NATIVE_WEBHOOK=1 KYC_NATIVE_IDEMPOTENCY=1`. El modo anterior sin esa flag no consulta stats. Se preserva el guard browser; las métricas son fetch server-to-server al fake19091 exacto, sin redirects y con timeout. El replay es APIRequest al BFF4219 exacto con cookies normales/Origin y sin Bearer añadido.

## Medición real

| Punto del recorrido | Observación |
| --- | --- |
| Después de guardar/releer/recargar la declaración | creation_requests no cambia respecto al baseline: **0creaciones** |
| Click sin consentimiento Didit | Alerta visible, contador no cambia: **0creaciones** |
| Inicio explícito tras consentimiento | POST BFF201, delta creation_requests **1** |
| Replay deliberado del mismo body y misma Idempotency-Key después de201 definitivo | POST BFF201, **mismo id** y **misma redirect_url** |
| Después del replay | Delta creation_requests continúa **1**, no hubo segunda llamada de creación al proveedor |

El replay no se usa ante resultado incierto ni como recuperación ciega. Body/key/IDs/URLs se conservaron solo en memoria y el reporte contiene únicamente statuses/booleanos/deltas. Además de no duplicar la fila local, esta prueba observa el límite externo falso para detectar una segunda petición de creación aunque Go devolviera el mismo registro.

También pasaron N08-D firma401/stale401/sesión404/válido200/replay200 y la autoridadInReview; retorno visible enrevisión sin aprobación, A/B/A y todos los logouts. SQL READ ONLY antes de cleanup confirmó **un intentoIN_REVIEW y un eventoPROCESSED/hash32** en el ámbito capturado por journal.

**probe_exit=0, harness_exit=0**, `TestNativeKYCAPIWithBFF` PASS11.59s (paquete12.659s con race).

## Cleanup y estado

Cleanup journaled PASS y finalizador directo COMMIT. Readback independiente READ ONLY: ledger90, SELECT/DELETE temporales revocados, ownerCREATEfalse; outbox50/pendientes0 estable. Preflight posterior de ambas cuentas/artefactos PASS, journal/markers ausentes, API18090 detenida. Los50ACK antiguos del adaptador se conservan sin borrado por patrón.

Fake/Next/proxy propios continúan retenidos solo para los siguientes gates N08; el fake mantiene en memoria su sesión sintética de este recorrido hasta el retiro del proceso, no se anuncia como almacenamiento remoto eliminado. No se llamó a Didit externo ni a worker general.

**N08-E cerrada para cero creación al guardar/sin consentimiento y replay de misma key/body sin nueva sesión.** No acredita carrera multiinstancia, otro idempotency-key, timeout incierto, reinicio del proceso fake/API ni factura real. La recuperación de pool y atomicidad N06 mantiene su evidencia separada. N08 global sigue parcial.

Siguiente slice en implementación source-only: variante de identidad sintética Unicode/Ñ/guion y RUN conDV-K minúscula para comprobar normalización y readback nativos, conservando estos gates de idempotencia/webhook. No se declara ejecutada ni se debilita dominio/seguridad por anticipado.
