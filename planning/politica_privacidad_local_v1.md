# Política `privacidad_local_v1`

Ratificada el 2026-10-08 para cuentas, archivos y operaciones **sintéticas del prototipo local exclusivamente**. No fija plazos legales ni una política productiva; no se aplica a documentos, dinero ni transacciones reales.

| Grupo | Regla local ratificada | Aplicación en LOCAL-PRIV-01A2 |
|---|---|---|
| Sesiones, tokens y credenciales | Revocar sesiones y tokens; eliminar hash vigente e historial al ejecutar baja elegible. La ventana de tres meses solo rige cuentas activas. | Eliminación transaccional junto a la minimización de cuenta. |
| Auditoría | Cinco años desde cada evento, solo campos estructurados autorizados. | Cada ejecución escribe el vencimiento en `retirar_en`; historial append-only por runtime. |
| Cuenta, perfil, preferencia y borradores | Retirar contacto, perfil, preferencia y texto libre sin finalidad; deshabilitar fixtures. Conservar ancla técnica mínima para FKs. | Minimización transaccional; sin cascada sobre reservas o registros de terceros. El correo persistido queda invalidado (`invalid.local`) y no permite contacto. |
| Solicitudes de derechos | Decisión, motivo y fechas mínimos por cinco años desde resolución; retirar contacto/texto libre innecesario. | La solicitud ejecutada recibe resultado estructurado y vencimiento calendario de cinco años. |
| Aceptaciones de términos | Versión, tipo y fecha mientras la cuenta está activa y cinco años desde la baja; no retener la versión compartida por cuenta. | Cada aceptación registra `retirar_en` desde el instante de inicio protegido de la baja. |
| Verificación sintética | Metadata mínima dos años desde estado terminal. Blob hasta 90 días del estado terminal o hasta baja elegible, lo primero. Borrado coordinado y recuperable. | `retirada_privacidad_en` registra la baja por separado. En casos ya terminales se conserva `resuelta_en` y su `retirar_en` originales; un caso pendiente se cierra al ejecutar la baja y su retención parte desde ese cierre. Cada blob entra en limpieza durable. |
| Cotizaciones no convertidas | Hasta 90 días tras expiración, o hasta baja elegible; las convertidas se conservan como snapshot de reserva histórica. | Las no convertidas se eliminan al ejecutar; las cotizaciones nuevas ya fijan `retirar_en=vence_en+90 días`. |
| Reservas, pagos fake, devoluciones y disputas | Mientras existan obligaciones abiertas; después, hechos mínimos 24 meses desde el último cierre terminal del conjunto relacionado. Reintentos/consultas no reinician plazo. Al vencer, retirar vínculos identificables sin otra finalidad ratificada. | Para reservas terminales de un titular se fija `vinculos_retirar_en` a 24 meses desde el máximo cierre existente de reserva, pago, devolución o disputa. El purgado/desvinculación al alcanzar esa fecha queda como seguimiento de retención, no se afirma completado por esta baja. |
| Mensajes sintéticos | Una baja elegible permite limpiar hilos terminales de reservas sintéticas donde participa el titular; preservar reservas, hechos financieros e historiales. | Elimina mensajes y cursores solo para reservas terminales, conservando el resto de los datos relacionados. Plazos productivos de mensajes pendientes. |
| Outbox | Mientras esté pendiente; 30 días desde entrega o fallo terminal. Avisos de credenciales sin finalidad se cancelan con motivo. | Cancela el aviso pendiente con `baja_local_sin_finalidad`; entregas de credenciales fijan retención de 30 días. No elimina trabajo pendiente silenciosamente. |

## Ejecución y controles

- Años/meses son calendarios. Se guardan el instante protegido de inicio y los vencimientos calculados con reloj inyectable.
- Solo un administrador autenticado ejecuta una solicitud de supresión del propio titular. No existe una operación para elegir otra cuenta.
- Al ejecutar, el backend bloquea la cuenta y vuelve a consultar reservas activas, pagos/devoluciones pendientes y disputas abiertas dentro de esa misma transacción. Las operaciones de esos dominios serializan con las filas de participantes. Un bloqueo devuelve códigos concretos y no minimiza la cuenta.
- Las escrituras de perfil, verificaciones, evidencias, borradores, tarifas, horarios y bloqueos manuales comparten el bloqueo de cuenta y vuelven a comprobar que siga activa dentro de su transacción. Si una carga sintética ya generó un archivo y pierde la carrera, la API lo elimina antes de responder.
- La ejecución es idempotente por solicitud/clave. Limpieza de blobs tiene trabajos durables; fallos mantienen la solicitud en limpieza pendiente, con reintentos limitados y recuperables al arrancar el backend.
- El resultado se denomina **baja local con minimización y retención residual**. No se afirma anonimización ni supresión integral mientras subsistan vínculos históricos.
- Las copias locales conocidas incluyen el volumen persistente `espacigo_pgdata`, secretos locales y almacenamiento privado de evidencias fuera del repositorio. Esta entrega no elimina ni inspecciona respaldos externos; una restauración local conocida debe reaplicar el registro de bajas. Ninguna copia se declara purgada.

## Pendientes de los padres

La operación de retención que retire vínculos transaccionales al cumplir `vinculos_retirar_en`, la política de fallos terminales de outbox y la evidencia de reaplicación en una restauración local requieren seguimiento. #185/#40 permanecen abiertas hasta cubrir sus criterios integrales. La supresión para información real, deberes legales, documentos reales y respaldos productivos sigue fuera de alcance; no se ejecuta GCP.
