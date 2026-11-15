# Privacidad Sandbox: estado informado y decisiones — 2026-11-10

## Fuente y alcance

El usuario aportó el texto de Datos y privacidad de la aplicación My Application, entorno Sandbox, identificador `806fe3f4-4c9b-4567-bf6f-ce8c3e688fce`. Posteriormente aclaró literalmente: «1 año automatico eliminar con la sesion estan selecionados por defecto en el sandbox».

Esta es evidencia informada por el usuario, no una lectura independiente de Didit ni una escritura realizada por el agente. Corresponde a configuración de aplicación, no a la definición de los nodos del workflow. No se inspeccionó ni modificó Producción.

## Estado informado, corregido

| Control | Selección informada por el usuario en Sandbox |
|---|---|
| Retención automática de datos de verificación | **1 año** |
| Consentimiento biométrico | **Automático** |
| Plantillas faciales tras eliminar una sesión | **Eliminar con la sesión** |

El texto aportado muestra Unión Europea (Irlanda) como región; según esa pantalla, la ubicación comprende Producción y Sandbox. Esto no acredita que ambos entornos compartan los demás ajustes.

La aclaración sustituye cualquier duda anterior sobre cuáles de esas tres opciones estaban seleccionadas. No acredita por sí sola una nueva operación de guardado ni un cambio a 30 días o a Siempre.

## Decisiones de producto autorizadas y estado de ejecución

El usuario pidió: «propone la mejor opcion y responde las preguntas que faltan confirmar dejalas como mi decision, continua». La propuesta queda adoptada como objetivo de configuración y política del proyecto, separada del estado observado:

1. **Respaldo automático del proveedor:** objetivo 30 días. Confirmar en el selector si está disponible ese valor; si ofrece únicamente «1 mes», registrar su semántica de calendario y no afirmar equivalencia exacta con 30 días. El estado informado sigue siendo **1 año**, sin cambio ejecutado. No inventar disponibilidad del selector ni contratar un plan.
2. **Consentimiento biométrico:** objetivo **Siempre**, manteniendo además el consentimiento separado de EspaciGo antes de transferir identidad. El estado informado sigue siendo **Automático**, sin cambio ejecutado. Esta decisión de producto no constituye asesoría o certificación jurídica.
3. **Plantillas:** mantener **Eliminar con la sesión**, ya seleccionado según el usuario. No habilitar retención de plantillas para detectar duplicados. No inferir eliminación de plantillas históricas a partir de esta selección.
4. **Borrado operativo:** mantener solicitud de eliminación de sesión tras verificación autoritativa, mediante outbox durable; un plazo automático de respaldo no es ACK ni prueba de borrado individual. No presentar la instalación local de V94 como cambio de configuración del proveedor.
5. **Correlación local de creación incierta sin session_id:** máximo 30 días desde el consentimiento original, sin prolongarlo por reintentos, con retiro dentro de las 24 horas siguientes al vencimiento. Conservar únicamente auditoría técnica mínima según el contrato aprobado de cinco años, indicando incertidumbre/borrado remoto no confirmado. No fabricar session_id, ACK ni prueba de eliminación. La implementación debe demostrar que la fecha original sigue disponible o introducir una transición append-only revisada; no reconstruirla desde updated_at.
6. **Preservación:** la reserva HMAC anti-reuso, la auditoría aprobada y el resultado mínimo autoritativo que sostiene la aprobación vigente son dominios distintos. No borrar el intento actual APPROVED mientras GetStatus dependa de él sin implementar primero un reemplazo seguro. No alterar V1–V94 aplicadas.

## Pendientes verificables

- Confirmar disponibilidad real del selector de retención y leer el valor guardado después de cualquier cambio autorizado; no atribuir al usuario una selección de 30 días o Siempre que no realizó.
- Sin operaciones online por el agente en este corte: no navegación, sesiones, capturas ni edición de la consola. El usuario conserva la comprobación online; los valores aportados no autorizan llamadas al proveedor ni captura de biometría.
- Continuar exclusivamente implementación y pruebas locales sintéticas de R2 bajo sus gates de fuente, backup, permisos, revisión y readback. La decisión anterior resuelve la elección de política; no acredita que el mecanismo de retiro ya esté implementado o aplicado.
- Fuera: datos/personas reales, certificación jurídica, publicación, integración a main, despliegue y cambios de Cloud SQL.
