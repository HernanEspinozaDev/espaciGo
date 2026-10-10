# LOCAL-ADMIN-01C — bloqueo restringido y reportes locales

Estado: **implementación para revisión**, vinculada a la Issue hija #233 de #111. No es una aceptación humana ni cierra M11. La matriz de cierre queda actualizada en `matriz_cierre_backend_local.md`.

## Alcance y trazabilidad

| Regla | Implementación | Evidencia enfocada | Límite que permanece |
|---|---|---|---|
| CU-43: bloquear/desbloquear con motivo estructurado, actor, fecha, correlación e historial append-only | V41 separa el estado de bloqueo del estado de cuenta, KYC y baja. Servicio local transaccional, auditoría de cinco años y grants de lectura/escritura separados. | `TestLocalAdminBlockRevokesSessionsAndAllowsOnlyRestrictedLogin`; integración PostgreSQL desechable. | No cubre todo el gobierno de cuentas de #112–#118. |
| Revocar sesiones/tokens anteriores; permitir nueva autenticación restringida | Login devuelve `restricted_mode`; el middleware deja pasar solo rutas de reservas propias/historial; la autorización de dominio vuelve a validar participante, estado y plazo. Bloquea operaciones comerciales y administrativas. Desbloquear revoca también la sesión restringida. | Prueba de sesión anterior revocada, token de acción invalidado, acceso normal rechazado, ruta permitida con contexto restringido, operación normal denegada y nueva sesión normal tras desbloqueo. | No altera reservas, contratos, ocupaciones ni fondos. No recupera sesiones antiguas. |
| Coordinar bloqueos con nuevas publicaciones, cotizaciones y reservas | La operación de bloqueo usa el bloqueo ordenado de las filas de cuenta. Escrituras de espacio y flujos de cotización/solicitud revalidan el bloqueo bajo el mismo bloqueo transaccional. | `TestAdminBlockSharesAccountLockAndPreventsLaterReservationGate`; integración del ciclo de reserva existente. | Solo rechaza nuevas operaciones; conserva obligaciones activas. |
| CU-45 parcial: reservas creadas en el período por estado actual | Agregado `[from, until)` sobre las reservas creadas; no reconstruye el estado histórico. | Reporte PostgreSQL cotejado contra estados distintos en la base desechable. | No completa el catálogo general CU-45. |
| CU-45 parcial: hechos financieros fake y obligaciones pendientes | Pagos de arriendo, devoluciones y autorización/captura/liberación de garantía quedan separados. Operaciones de garantía no se presentan como ingreso. Las obligaciones pendientes se devuelven en otra colección; eventos fake recuperables sin aplicación quedan como conciliación pendiente. | Reporte PostgreSQL comparado con ledger; reintento idempotente de pago antes del agregado no aumentó el movimiento reportado. | Sin comisión, liquidación, boleta ni proveedor real. |
| Período explícito, mínimo UTC, máximo 31 días calendario e IANA | Handler valida RFC3339, offset compatible con la zona IANA y rango calendario de 31 días; el servicio devuelve instantes UTC y conserva el identificador IANA. | `TestReportPeriodRequiresExplicitIANAAndCalendarBoundaries`; `TestPeriodUsesCalendarDaysInExplicitZone`; `TestReportPeriodNormalizesInstantsToUTCButKeepsIANAZone`, incluyendo transición DST. | No incorpora búsqueda textual ni reportes históricos. |
| Privacidad y auditoría de consulta | Las respuestas solo exponen agregados; cada lectura válida escribe un evento mínimo de auditoría por petición, no por fila. CORS y errores conservan el contrato HTTP común. | Integración consulta el endpoint de reservas con un administrador autenticado y prueba errores 401/403 con `request_id` coincidente y `WWW-Authenticate: Bearer` para 401. | La cobertura transversal de productores y el outbox común siguen pendientes. |
| Mock | Controles integrados para bloquear, desbloquear y consultar los dos reportes; los datos se borran al perder sesión y las respuestas viejas se descartan cuando cambian los criterios o la sesión. | `npm --prefix mock run build`; `node --test mock/test/admin-local-state.test.mjs`. | Mock local; no es el frontend definitivo. |

## Validación ejecutada

- Migraciones aplicadas desde cero en PostgreSQL/PostGIS desechable con red aislada y almacenamiento temporal; V41 se migró y los tests de integración utilizaron `espacigo_runtime` donde corresponde. El volumen de desarrollo persistente y sus secretos no se montaron ni modificaron.
- Integración administrativa enfocada: bloqueo/desbloqueo, revocación de sesión y token, sesión restringida, acceso por rol, auditoría, reportes, período DST y serialización con bloqueo de cuenta: **pasó**.
- Integración de reservas en PostgreSQL desechable con agregación de reportes, obligación de resultado fake sin evento y reintento idempotente: **pasó**; comprobó que la lectura no concilia ni altera la operación pendiente.
- `go test ./internal/adminlocal/... ./internal/identity/... ./internal/identity/transport/http ./cmd/api`: pasó; `go vet` sobre los mismos paquetes y `cmd/api`: pasó.
- `node --test mock/test/admin-local-state.test.mjs`: 1/1 pasó; `npm --prefix mock run build`: pasó.
- `python3` con `yaml.safe_load` validó `planning/openapi.yaml` y verificó las cuatro rutas de esta subentrega; `git diff --check`: pasó.
- Se omitieron: suites completas no afectadas, navegador de aceptación del prototipo y cualquier prueba o despliegue GCP. No se requieren para revisar este corte.

## Pendientes explícitos

CU-45 queda aceptado solo en los dos reportes locales descritos. Continúan abiertos #111–#118 por los criterios transversales de permisos, productores de auditoría, outbox común, administración y reportes generales. No se declara completo M11 ni se desbloquea LOCAL-CORE-02. Los padres #109/#110, #185/#40 y las obligaciones de proveedor real siguen abiertos. No se incorpora GCP.
