# LOCAL-KYC-01 — elegibilidad sintética local

**Fecha:** 2026-10-08 · **Trazabilidad:** Issue hija #200 de #45, relacionada con #48–#50; decisión en D-KYC/LIST. Esto amplía el prototipo local y no completa M03.

## Decisiones ratificadas

- Una aprobación sintética vigente concede elegibilidad local únicamente para el tipo aprobado (`kyc` o `kyb`). No concede roles.
- Una solicitud posterior, pendiente o rechazada no retira una aprobación anterior. El rechazo pertenece a su caso.
- La elegibilidad se conserva hasta una revocación administrativa explícita, con motivo estructurado, auditoría append-only y trazabilidad. La revocación bloquea nuevas operaciones dependientes, pero no modifica reservas previas ni impide consultar historial. Una aprobación nueva y explícita puede restablecer elegibilidad.
- Un tipo no sustituye automáticamente al otro. No se calcula el permiso tomando solamente el caso más reciente.
- Los estados y las evidencias siguen siendo sintéticos. No se aceptan documentos/RUT ni proveedores reales; #142 mantiene ese alcance separado.

## Diseño implementado

V28 añade campos de subsanación/revocación a `verificacion`, historial append-only ordenado por secuencia y una proyección persistente por cuenta/tipo. La migración no depende del rol `espacigo_runtime`; `dbbootstrap` concede al rol de aplicación solo lectura/inserción sobre historial, uso de secuencia y CRUD de elegibilidad. El backfill registra solicitudes y la resolución vigente observable, sin inventar transiciones que los datos anteriores no conservan.

La revisión, reintento y revocación comparten el bloqueo de cuenta con la baja local. Cada transacción revalida el caso después de tomar el bloqueo, lee el reloj inyectable después de adquirir los bloqueos y registra estado, historial, elegibilidad y auditoría en la misma transacción. La API expone consulta de casos, historial y elegibilidad propia, corrección tipada/reintento idempotente, revisión y revocación administrativa idempotente.

La corrección de un rechazo se limita al código correspondiente: `documento_vencido` → `fixture_vigente_actualizado`; `antecedentes_incompletos` → `antecedentes_fixture_actualizados`; `inicio_actividades_no_confirmado` → `inicio_actividades_fixture_actualizadas`.

Los reintentos anteriores a V28 no contenían el código de corrección. La migración los conserva con `legado_pre_v28_sin_codigo`; ese marcador documenta la ausencia histórica y no se acepta para crear un reintento nuevo.

## Mapeo ratificado e integración por corte

El 2026-10-08 se ratificó que las cuentas personales actuales requieren KYC sintético aprobado para nuevas publicaciones y reservas. KYB no sustituye KYC en este recorrido y no se requieren ambos tipos. La elegibilidad no concede roles. La solicitud de reserva revalida KYC vigente para anfitrión y arrendatario dentro de la transacción, después de bloquear ambas cuentas; aprobación y baja usan el mismo bloqueo. Una baja efectiva retira la elegibilidad en esa misma transacción, registra historial y conserva los vencimientos terminales originales.

El PR #205 implementó el endpoint local owner-only para publicar/ocultar/reactivar con el gate KYC sintético efectivo y un historial propio. Esto acepta ese tramo, no el ciclo general M04: el estado `activa` aún no se conecta al catálogo general, y edición/galería permanecen pendientes. #45/#48–#50 siguen abiertas por sus criterios restantes.

## Pendientes

Notificación de resultado KYC en un outbox de dominio aún no está implementada; Mailpit/proveedor y la notificación durable ratificada en AUTH aplican al cambio de credenciales, no se atribuyen a esta revisión. La elegibilidad y el historial derivado siguen el vencimiento original de metadata KYC, dos años desde el estado terminal; la baja los retira sin reiniciar la fecha. El ZIP propio incluye verificación, historial y elegibilidad efectiva sin identificar al revisor. La purga física coordinada al vencimiento sigue pendiente en el cierre de privacidad. La integración, consentimiento y retención productivos permanecen en #142. Los criterios generales de #45 y #48–#50 continúan abiertos.
