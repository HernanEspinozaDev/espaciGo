# F5 — contrato de identidad documental para contratación

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD base `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`  
**Estado:** corte parcial de dominio/contrato sintético con gate preflight conectado al servicio de reservas; no hay adaptador persistente ni cierre de F5.

## Contrato implementado

- Se añadió el paquete independiente `internal/identityverification`, separado del login/sesión (`internal/identity`) y del KYC/KYB fixture (`internal/verification`).
- `Service.RequireForBooking` solo satisface el gate cuando anfitrión y arrendatario son cuentas distintas y ambas tienen una verificación de identidad documental aprobada mediante revisión manual, con `verified_at` no futuro, `expires_at` estrictamente posterior al instante comprobado y `revoked_at` ausente. El repositorio debe definir `expires_at` como el mínimo entre el vencimiento del documento y los 24 meses desde la revisión. Interpretación fail-closed: cualquier `revoked_at` no nulo invalida la verificación, aunque esté en el futuro; este contrato no admite revocaciones programadas.
- La aserción de dominio guarda solo sujeto, clase de verificación, decisión, método de revisión y vigencia/revocación. No contiene archivos, números de documento, biometría, nombres originales ni claves de almacenamiento.
- La elegibilidad KYC/KYB `local-fixture-v1` no constituye identidad documental, aunque aparezca aprobada y vigente.
- No se cambió la política de publicación: sigue independiente de identidad/documentos del espacio.

## Pruebas y ejecución

- RED real del gate booking: con el `Decide(aprobar)` anterior, una reserva se aprobaba aunque no hubiera proveedor de identidad. La prueba ahora demuestra bloqueo cuando falta el verificador; la falta de identidad vigente devuelve el error contractual distinto.
- RED real HTTP: los nuevos errores inicialmente caían en 500 genérico; el mapeo explicitado quedó en 409 `identity_verification_required` y 503 `identity_verification_unavailable`.
- GREEN focalizado: tests de `internal/booking` conectan el `identityverification.Service` real a un repositorio de reservas sintético; prueban ambas identidades vigentes, ausencia separada del host o arrendatario, expiración del host, revocación del arrendatario, IDs de reserva/participantes malformados o incoherentes, ausencia/error del verificador, cancelación/deadline y rechazo sin gate. Pruebas HTTP cubren 409/503 por la ruta real de aprobación y una ruta real de rechazo válido sin verificador que llega al repositorio.
- Suite Go final con DSN de integración unset: `env -u TEST_DATABASE_URL -u DATABASE_URL -u IMG01C_TEST_DATABASE_URL -u IMG01D_TEST_DATABASE_URL -u IMG01F_TEST_DATABASE_URL go test -count=1 ./...` — PASS; `go vet ./...` y `git diff --check` — PASS. Sin conexión DB, los tests de integración PostgreSQL no acreditan este flujo persistido.
- Frontend `pnpm api:check` — PASS; inventario y tipos OpenAPI actuales, 182 operaciones.
- No se inició API, DB, scanner ni otro servicio; no hubo documentos, cuentas o credenciales reales.

## Revisión independiente — 2026-10-28

- La revisión inicial `deleg_37aca04f` señaló semántica de `revoked_at` futuro y huecos de cobertura; seguimiento de contrato `deleg_5d12f524` pasó tras documentar el criterio y ampliar pruebas. Confirmó que la política aprobada contempla estado revocado y no define revocación programada; rechazar cualquier `revoked_at` presente es fail-closed razonable. Verificó las pruebas de bordes y la allowlist de campos minimizados.
- Revisión read-only de integración `deleg_589b7cf2`: `passed=true`, sin hallazgos concretos sobre el contrato de servicio, scope de participante o mapeo 409/503. El follow-up de pruebas `deleg_baaee019` no encontró defectos de producción, pero reportó una brecha de cobertura: faltaba rechazo válido por HTTP sin verifier y UUID malformado en IDs de reserva/anfitrión. Se añadieron ambos casos junto a la verificación de que el rechazo llega al repositorio; la re-revisión independiente `deleg_fed45d74` pasó sin findings ni gaps.
- Límites aún confirmados por esa revisión: `cmd/api` no cablea repositorio PostgreSQL documental, y `Get`→gate→`Repository.Decide` no comparte transacción; runtime no se probó y la revocación podría competir con aprobación cuando exista persistencia.

## Límites que mantienen F5 parcial

- `booking.Service.Decide(aprobar)` consulta la reserva para obtener anfitrión/arrendatario y requiere el gate documental explícito antes de delegar la decisión. Dependencia ausente o error de proveedor falla cerrado con 503; identidad no vigente devuelve 409. `rechazar` permanece independiente.
- `cmd/api` aún no instancia/adapta el repositorio documental ni cablea el verificador; el servicio runtime queda fail-closed y devuelve 503 al intentar aprobar hasta que exista esa persistencia. El repositorio PostgreSQL actual aún conserva su KYC fixture en la creación de solicitudes; KYC no equivale a la política documental nueva.
- El preflight `Get` → verificación → `Repository.Decide` atraviesa límites de repositorio distintos y no es atómico con el commit de la reserva: falta bloquear/revalidar vigencia y revocación dentro de la transacción final. No se afirma contratación persistida ni protección contra carrera de revocación.
- No hay adaptador/repositorio documental persistente, intake en cuarentena, scanner real fail-closed, revisión manual operativa, ACL, distintivo, revocación/vencimiento con datos, retención, borrado, UI ni recorrido BFF. Las pruebas son unitarias/sintéticas.
- El siguiente corte debe diseñar/cablear la persistencia documental y su coordinación transaccional sin mapear KYC antiguo ni alterar la independencia de publicación; no aplicar DDL ni iniciar API/DB hasta que F6 confirme target sintético aislado autorizado.
