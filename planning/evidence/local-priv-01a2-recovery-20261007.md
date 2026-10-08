# LOCAL-PRIV-01A2 — recuperación y reanudación

Fecha: 2026-10-07. Evidencia complementaria para PR #191. Se conservaron la base persistente, secretos, cuentas y datos históricos; solo se modificaron las credenciales de cuentas sintéticas de prueba autorizadas. No se ejecutó supresión ni trabajo de GCP.

## Cambios comprobados

`scripts/verify-local-privacy-dispute.py` ahora:

- Reutiliza las cuentas guardadas fuera del repositorio. Si el inicio de sesión falla, comprueba el estado de esa cuenta sintética: intenta registrarla si no existe, reemite la verificación por Mailpit si quedó en `correo_pendiente`, o restablece por Mailpit la contraseña local desactualizada de una cuenta activa. Guarda la credencial generada con permisos `0600` y no la imprime.
- Persiste claves idempotentes antes de las mutaciones. Tras una interrupción, consulta la incidencia, las solicitudes propias y el estado de reserva para recuperar commits remotos antes de repetir operaciones. Las evaluaciones se repiten con sus mismas claves idempotentes.
- Comprueba que existe la fila `schema_migrations.version = 24`; no requiere que V24 sea la versión máxima.

## Ejecuciones

- Se reemplazó deliberadamente la contraseña local del anfitrión por un valor inválido. `bash scripts/verify-local-privacy-dispute.sh` la recuperó mediante el flujo de correo local y terminó el recorrido.
- `LOCAL_PRIV189_TEST_INTERRUPT_AFTER=dispute_open bash scripts/verify-local-privacy-dispute.sh` interrumpió el proceso tras el commit remoto y antes de guardar el ID local. Al reanudar sin la variable, el flujo terminó con una sola incidencia para la reserva interrumpida: `8b10e355-e455-4cea-bf79-653f5a80c9d5`, posteriormente cancelada por API.
- `LOCAL_PRIV189_TEST_INTERRUPT_AFTER=rights_request_host bash scripts/verify-local-privacy-dispute.sh` interrumpió tras crear la solicitud del anfitrión y antes de guardar su ID. Al reanudar, el script recuperó el registro de la lista autenticada; anfitrión y arrendatario tenían una solicitud cada uno en ese recorrido y este terminó correctamente.
- Última ejecución completa: reserva `81a6e9ae-434b-4662-ad9d-d614925a8644`, incidencia `53ed3bb6-7705-41b5-9853-944026939754`. Se mantuvo la separación anfitrión/arrendatario/administrador y la evaluación de supresión siguió sin ejecutarse.
- V24 está aplicada (`EXISTS = true`; máxima actual 24). Para comprobar que la condición no depende de ser la última versión, se insertó temporalmente una fila V999 con transacción y se consultó `EXISTS(version=24) AND max(version)>24`; dio `true`. Se revirtió la transacción y el máximo volvió a 24.
- `python3 -m py_compile scripts/verify-local-privacy-dispute.py` y `git diff --check` pasaron.

El hook de interrupción es únicamente para repetir esa evidencia y acepta `dispute_open`, `rights_request_host` o `rights_request_renter`. Para completar la reanudación, ejecutar el comando de nuevo sin `LOCAL_PRIV189_TEST_INTERRUPT_AFTER`.

## Reintento de token inválido — 2026-10-08

El consumidor de mensajes ahora trata la respuesta común `422 invalid_token` (token vencido o ya consumido) así: guarda el ID del mensaje descartado en el archivo de estado local, solicita un token nuevo por el endpoint normal, excluye el mensaje anterior y reintenta hasta dos veces. Si el Backend responde `429` u otro error, el script se detiene sin eludir sus límites. No se cambió ninguna expiración.

La comprobación integrada usó `LOCAL_PRIV189_TEST_CONSUME_TOKEN_FIRST=recovery`: en una cuenta anfitriona sintética renovada, el hook consumió el token real de Mailpit antes de que el flujo normal lo procesara. La primera operación normal recibió `invalid_token`; se descartó el correo, se pidió un token de recuperación nuevo y la cuenta completó el flujo y el recorrido de disputa. La reserva de esa ejecución fue `b4694e6e-704d-4f26-9545-a5eb2ec163fa` y su incidencia `6bce980a-538f-44da-93ef-5f57cb227922`. El ID descartado quedó registrado en el estado externo con permisos `0600`; ni token ni contraseña aparecieron en la salida.

Se ejecutó `python3 scripts/test_verify_local_privacy_token_retry.py`: dos pruebas pasaron. Una confirma descarte persistido, petición y consumo de correo nuevo después de `invalid_token`; la otra comprueba el máximo de dos reintentos y que se detiene al agotarlos. Ambas se ejecutan en milisegundos y no esperan expiraciones. También pasaron `python3 -m py_compile scripts/verify-local-privacy-dispute.py scripts/test_verify_local_privacy_token_retry.py`, `git diff --check` y una ejecución normal posterior del verificador (reserva `9c2a2c6e-b754-41cc-9274-9c05fd3750e7`, incidencia `42b49774-a776-4879-b02f-c833ac6c8976`).

Para aislar el límite de correos del anfitrión usado en ensayos previos, se renovó su identidad sintética local y se conservó el puntero anterior en el archivo privado; los registros y fixtures anteriores permanecen en PostgreSQL. La pareja renter/admin, el volumen y los secretos no se cambiaron.
