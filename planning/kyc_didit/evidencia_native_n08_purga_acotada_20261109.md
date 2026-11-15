# N08-N — purga nativa acotada bajo rol worker (2026-11-09)

## Criterio y entorno

C10 canónico exige purga SQL acotada a100 filas bajo `espacigo_privacy_cleanup_worker`. La cobertura histórica efímera se conserva, pero no sustituye este gate en el destino seleccionado.

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty. Único destino nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, socket `/var/run/postgresql`, OID16388, PostgreSQL180006, ledger91. No cuentas nuevas, provider externo ni worker general.

## Diseño revisado y ejecución

`db/repair/test_native_kyc_n08_bounded_purge.sql` es un ensayo **BEGIN/ROLLBACK**, no migración ni limpieza operacional. Revisión independiente `deleg_79843ba8`: PASS source-only con límites de ledger/rol explícitos. Preflight parental separado compara ledger completo con fuente y autoriza el fixture desde la política privada; el script exige versión91/checksum además del destino/administrador exactos.

- Antes del SQL se persistió un journal privado exclusivo0600 con la cuenta sintética autorizada y101 UUIDs canónicos/distintos; directorio0700 y fsync. No insertar en `public.usuario` ni copiar credenciales.
- Locks de revisiones/attempts impiden cambiar elegibilidad mientras dura el ensayo; timeoutlock3s y statement10s. Guard global exige cero revisiones ajenas elegibles o que venzan dentro de2min. Readback previo confirmó0. No reclamar trabajo ajeno.
- Se insertan101 revisiones sintéticas ya vencidas, con IDs exactos del journal, dentro de la transacción; ciphertext de29bytes solo satisface constraints. **No representa cifrado válido de aplicación**, ya acreditado por N03 y pruebas crypto separadas.
- Fixture se prepara como administrador peer. Las dos llamadas a la función real se ejecutan mediante `SET LOCAL ROLE espacigo_privacy_cleanup_worker`, rol NOLOGIN restringido con EXECUTE y función SECURITY DEFINER del owner. Esto no acredita login de worker, scheduler ni operación periódica.
- Primera llamada retorna100; el script exige exactamente una fila journaled restante. Segunda retorna1; el script exige0. Cualquier discrepancia aborta. Final **ROLLBACK**, no COMMIT de fixtures ni de purgas.

Ejecución parental completa por stdin en terminal Hermes: BEGIN, guards/locks, INSERT101, SET/RESET ROLE de ambas llamadas, checks DO y ROLLBACK; exit0. Readback independiente en conexión nueva READ ONLY confirmó `fixture_revisions_remaining=0`, `fixture_account_artifacts=0`; preflight completo de ambas cuentas PASS.

## Incidentes preservados

- Un primer launcher usó `psql \i` hacia un checkout que el usuario postgres no puede recorrer: permiso denegado antes de BEGIN. Se trasladó el script exclusivamente a stdin, sin cambiar permisos del checkout.
- Una lectura programática de archivo truncó una línea larga de la allowlist privada: el script abortó antes del INSERT de revisiones. Se reconstruyó desde el journal completo, sin imprimir IDs. La transacción previa se revirtió al desconectar; no se crearon filas persistentes.
- `sudo id` directo se usa al inicio del mismo comando foreground antes de psql con stdin: evita que la entrada SQL interfiera con la autenticación de entorno Hermes. No se leyó/imprimió/pasó `SUDO_PASSWORD` a hijos ni se pidió contraseña al usuario.

Backup previo privado `.local/testing/kyc-native/pre-n08-bounded-purge-20261109.dump`, custom format, 8324803bytes,0600/ext4, SHA-256 `6be1f6461d5dafe27ba6ffb614a64ba2c18e3cc8287323329327cd82b5133106`. Listado y parse completo pg_restore PASS. Journal y wrappers privados quedan como recibos de rollback/readback; no se convierten en cuenta nueva ni en autorización para otra base.

## Resultado y límites

**N08-N PASS nativa rollback-only:** la función real bajo rol worker limitado procesa100+1 de101 filas propias, sin tocar filas ajenas y sin residuos tras rollback. No demuestra una purga persistente programada, duración real de30d, SLA, retención legal aprobada ni borrado remoto real. K/L y N06 acreditan separadamente commit/purga post-aprobación, outbox durable/retry/ACK y fechas de retiro.
