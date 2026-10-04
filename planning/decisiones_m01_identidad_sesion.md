# M01 — decisiones ratificadas de identidad, sesiones y tokens

**Estado:** decisiones de producto ratificadas por el usuario el 2026-10-04; registro de autorización para AUTH-ARCH-01, AUTH-DB-01 y AUTH-DB-02. No es una migración, no modifica ES1/ES2 y no acredita implementación. Las reglas que no están fijadas por requisitos se atribuyen a esta autorización explícita, no a una inferencia del agente.

## 1. Correo e identidad

- Conservar el correo recibido como `correo_original` y guardar por separado `correo_normalizado`.
- Primero validar el formato del valor ingresado (ES1 RQF-002). Después, derivar la clave canónica eliminando espacios externos y aplicando Unicode `casefold` independiente del idioma a toda la dirección. No añadir NFC/NFKC ni otras transformaciones no autorizadas.
- Comparar identidad sin distinguir mayúsculas. Aplicar exactamente el mismo algoritmo en registro, login y recuperación; imponer `UNIQUE` sobre `correo_normalizado` en PostgreSQL, no confiar en una consulta previa de aplicación.
- No eliminar puntos, sufijos `+alias` ni aplicar reglas propias de proveedores.

**Trazabilidad:** ES1 `B_requerimientos_funcionales.md` RQF-002 y RQF-007; `D_casos_de_uso.md` CU-01, pasos 3 y 7, y flujo A1; ES2 `B_diccionario_datos.md` B.2, tabla `usuario`, `correo_normalizado` (UK sobre normalización/casefold). La conservación de `correo_original` y el algoritmo exacto de normalización son decisiones de producto ratificadas aquí, no reglas completas que ya estuvieran especificadas en esos campos.

## 2. Respuestas públicas y conflicto de enumeración

- **Registro duplicado:** conservar literalmente el comportamiento de CU-01 A1: rechazar el alta e informar que el correo ya está registrado, recomendando iniciar sesión o recuperar la contraseña. No cambiar silenciosamente ES1 ni ocultar este mensaje en AUTH-ARCH-01.
- **Conflicto a revisar antes de cualquier propuesta de cambio:** revelar el duplicado facilita la enumeración de cuentas, mientras que el control de amenaza de AUTH-ARCH-01 recomendaba respuestas equivalentes. En este trabajo se preserva CU-01 A1 por autorización del usuario; no se propone cambiarlo. Cualquier reconciliación futura requiere un cambio de requisito trazable y revisado.
- **Recuperación:** responder de forma genérica, sin revelar si existe la cuenta. Esto se aparta de CU-05 A2, que actualmente indica que el sistema informe que no puede continuar con un correo no registrado. Mantener la discrepancia visible: la decisión ratificada rige el diseño, pero no reescribe el texto fuente de ES1.

**Trazabilidad:** ES1 CU-01 A1 y RQF-007; ES1 CU-05, flujo principal y A2; amenaza de enumeración documentada en `invariantes_identidad_sesion.md`. Las respuestas de registro y recuperación tienen tratamientos distintos por decisión explícita.

## 3. Sesiones

- Mantener RNF-016: invalidar tras 30 minutos sin actividad de usuario y, en todo caso, a las 8 horas de la creación.
- Persistir `ultima_actividad_en` y la fecha de vencimiento absoluto; en el diseño de AUTH-DB-01, `expira_en` representa `creada_en + 8 horas`. La expiración inactiva se evalúa como `ultima_actividad_en + 30 minutos`.
- El Backend valida ambas expiraciones con su propio reloj. Solo las solicitudes autenticadas de operaciones de usuario renuevan la última actividad. Health checks, polling automático, keepalives y renovación de tokens no la renuevan.
- Rechazar una sesión si venció cualquiera de los dos plazos. Ninguna actualización de actividad puede extender ni sobrepasar `expira_en` absoluto.

**Trazabilidad:** ES1 RNF-016 (30 minutos de inactividad; máximo absoluto 8 horas), RQF-018 (sesión temporal) y RQF-023 (cerrar sesión); ES2 B.2, tabla `sesion` (`creada_en`, `expira_en`, `revocada_en`). La persistencia de `ultima_actividad_en` y los eventos que cuentan como actividad quedan autorizados por este mensaje.

## 4. Tokens

- Tokens aleatorios, de un solo uso, persistidos solo mediante hash y excluidos de logs.
- Una emisión nueva invalida los tokens anteriores aún activos del mismo usuario y propósito. ES2 B.2 no representa explícitamente esa invalidación; AUTH-DB-01 propone `invalidado_en` como marca de persistencia derivada de la decisión ratificada, sin cambiar la conducta del producto.
- Recuperación de contraseña: TTL de 15 minutos, exigido por RQF-021 y CU-05.
- Verificación de correo: TTL de 24 horas; decisión de producto porque ES1 fija la verificación (RQF-008–010/CU-02), pero no ese plazo.
- Máximo 5 intentos fallidos por token. Máximo 3 emisiones/reenvíos por cuenta y propósito en una ventana móvil de una hora; cada token efectivamente emitido cuenta, incluso si después se invalida. Aplicar además límites por IP en Backend. Estos límites no bloquean el login ni activan su bloqueo por cuenta.
- El bloqueo de login continúa siendo independiente: RQF-014/015/017 define contador, 5 intentos fallidos consecutivos y liberación tras 30 minutos.
- Cambio de correo: TTL de 15 minutos solo si un flujo de confirmación ya está requerido. No se encontró flujo de cambio de correo en los requisitos ES1; por tanto no agregar ese flujo ni el propósito `cambiar_correo` a la migración M01. ES2 B.2 actualmente incluye ese propósito en `token_accion`; queda como discrepancia de diseño que no autoriza por sí sola un flujo nuevo.

**Trazabilidad:** ES1 RQF-008–010/CU-02; RQF-019–022/CU-05; RQF-014/015/017; ES2 B.2, tabla `token_accion` (`proposito`, `token_hash`, `creado_en`, `expira_en`, `consumido_en`, `intentos`). Los TTL de verificación, límite por token, emisiones y reenvíos son valores de producto ratificados aquí. Los umbrales numéricos por IP no fueron fijados en este mensaje y no se inventan en el esquema.

## 5. Brechas DB02-09 y efecto en AUTH-DB-02

Las tres brechas siguen abiertas. La autorización de correo, sesiones y tokens no las resuelve.

| Requisito | Brecha vigente | Efecto real en la migración inicial AUTH-DB-02 | Tratamiento autorizado |
| --- | --- | --- | --- |
| RQF-213 — preferencia de uso | ES1 CU-01/RQF-213 pide guardar una preferencia no excluyente; `usuario` no tiene ese campo en ES2, y `rol_usuario` modela permisos, no la preferencia. | Afecta la cobertura de persistencia del registro. No forma parte de las seis tablas M01 definidas por Anexo B, pero la migración de esas tablas no debe presentarse como si ya cubriera la preferencia. | Mantener abierta y trazada. No añadir columna/tabla ni equiparar preferencia a rol sin el ticket/decisión de modelo correspondiente. |
| RQF-217 — historial de contraseñas | CU-50 exige rechazar claves usadas en los últimos 3 meses; Anexo B no define entidad/campos de historial ni política de minimización/retención. | Afecta el soporte de persistencia de CU-50; el esquema base de seis tablas no puede hacer cumplir esta parte del requisito por sí solo. No impide crear esas seis tablas si el alcance se limita a su contrato aprobado, pero no se puede afirmar cobertura de RQF-217 ni agregar DDL de historial ahora. | Mantener abierta y trazada. Requiere ticket/decisión de retención y minimización antes de DDL del historial. |
| RQF-218 — notificación de cambio | CU-50 requiere notificar; el diccionario no define un evento M01. `notificacion`/`entrega_notificacion` y `outbox_evento` corresponden a capacidades comunes con ownership M09/M11. | No requiere columna ni tabla nueva en las seis tablas de la migración AUTH-DB-02. Sí deja pendiente el contrato de evento e integración para satisfacer el comportamiento de CU-50. | Mantener abierta y trazada con M09/M11. No duplicar tabla de notificación ni declarar resuelta la entrega por la existencia de estructuras genéricas. |

**Fuentes del análisis:** `planning/revision_diccionario_datos.md`, crosswalk RQF-213–218 y DB02-09; ES1 RQF-213/217/218 y CU-01/CU-50; ES2 B.2 y mapa de ownership/MAP-03. Esta clasificación limita el alcance de la migración base; no cierra las brechas ni amplía la aceptación de AUTH-DB-02.

## 6. Artefactos y estado

- AUTH-ARCH-01 debe reflejar estas reglas en sus invariantes, flujos y amenazas y mantener DB02-09 como brechas abiertas.
- AUTH-DB-01 debe derivar el modelo de seis tablas sin contradecir Anexo B ni ocultar las ampliaciones autorizadas para `correo_original` y `ultima_actividad_en`; el artefacto de persistencia correspondiente es `planning/persistencia_identidad_sesion.md` y se publica en un PR separado.
- AUTH-DB-02 continúa `blocked` hasta la fusión/revisión de sus prerrequisitos y disponibilidad de la instancia PostgreSQL desechable autorizada. No se ejecutó ni creó DDL al documentar estas decisiones.
- No se modificó Kanban ni su base de datos.
