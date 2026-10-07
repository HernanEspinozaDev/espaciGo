# Evidencia LOCAL-AUTH-01 / LOCAL-CORE-01

Fecha: 2026-10-07. Rama: `codex/local-l1-plan-core`, basada en el merge de #178 (`716a4a38512ddbd693d1541620786a75d20432ca`). Esta evidencia corresponde al cambio preparado para revisión; no supone aceptación de #182, #180 ni cierre de M01/L1.

## Entrega verificada

- Registro conserva la preferencia inicial `ofrecer` o `arrendar` como dato no excluyente; no concede roles.
- Cambios de contraseña y recuperación rechazan contraseñas vigentes durante los tres meses calendario anteriores, contando desde que cada hash dejó de ser actual. Los hashes vencidos se purgan; el hash vigente se conserva mientras la cuenta lo necesite.
- Actualización de contraseña, historial, revocación de sesiones, auditoría mínima y evento de aviso quedan en una transacción. El aviso persistente se procesa con reintentos y deduplicación por el worker local, usando Mailpit como transporte de desarrollo. El correo no incluye contraseña, hash ni token.
- La auditoría local registra actor, recurso, acción, resultado, motivo estructurado, fecha y correlación; el rol de aplicación no puede actualizar ni borrar esos registros. Su retiro se fija a cinco años del evento. Esto no implementa inmutabilidad productiva de RNF-017.
- OpenAPI y el formulario del mock envían la preferencia de onboarding.

## Comprobaciones ejecutadas

- `python3 scripts/check_local_requirement_trace.py`: cobertura exacta de los catálogos; 236 RQF, 43 RNF, 52 CU y 35 HU.
- `git diff --check`: sin errores.
- `go vet ./internal/identity ./internal/identity/transport/http ./internal/adapters/postgres/identity ./internal/adapters/devauth ./cmd/api`: correcto.
- `cd mock && npm run build && node --test test/*.test.mjs`: compilación correcta; 33 pruebas, 33 pasaron.
- `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity ./internal/migrator ./internal/identity ./internal/identity/transport/http ./internal/adapters/devauth ./cmd/api`: correcto. Las pruebas PostgreSQL se ejecutaron en la instancia desechable del script. Incluyeron migraciones desde cero, expiración exacta de tres meses, rechazo/aceptación de reutilización, concurrencia de cambios, rollback forzado de auditoría, recuperación y deduplicación del outbox, y permisos mínimos de `espacigo_runtime`.
- Seguimiento PR #183: `python3 -m py_compile scripts/verify-m01.py scripts/verify-m02.py` pasó. Ambos consumidores incluyen `use_preference`; `verify-m01` también conserva un caso explícito de omisión que espera 422.
- Seguimiento PR #183: pruebas PostgreSQL enfocadas ejecutadas en otra base desechable con el script anterior. Bajo un bloqueo real de fila, el reloj avanza hasta el vencimiento de la sesión/token; ambos cambios se rechazan sin actualizar la contraseña, consumir el token, revocar parcialmente la sesión ni insertar historial/outbox/auditoría. Una tercera prueba comprueba que historial, expiración, auditoría y aviso usan el instante en que se procesa el cambio tras liberar el bloqueo.

No se migró ni se limpió `espacigo_pgdata`; no se leyeron, modificaron ni publicaron secretos; no se ejecutaron pruebas o despliegues GCP. Las pruebas de transporte usan SMTP falso. La entrega no afirma que se haya comprobado el envío a una instancia Mailpit activa.

## Pendientes conservados

- #31/#34 y los requisitos generales M01 permanecen abiertos hasta la aceptación de sus criterios completos.
- #74 mantiene pendiente proveedor real/sandbox, credenciales y contrato; #76/#78/#79 siguen sujetos a sus dependencias reales.
- Los consumidores de notificación distintos del aviso local de credenciales requieren la decisión D-COMM. RNF-017 productivo permanece pendiente.
- La aceptación de esta implementación local no cierra los padres incompletos ni completa el cierre total del backend local.
