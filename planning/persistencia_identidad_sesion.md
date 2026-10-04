# Diseño de persistencia de identidad y sesión (AUTH-DB-01)

## Alcance y autoridad

Contrato físico propuesto para las seis tablas M01 del Anexo B ES2 v2.0: `usuario`, `rol_usuario`, `sesion`, `token_accion`, `version_terminos` y `aceptacion_terminos`. `perfil_usuario` pertenece a M02. No se crea DDL en esta tarea. Las decisiones M01 ratificadas por el usuario el 2026-10-04 están registradas en `planning/decisiones_m01_identidad_sesion.md`; este artefacto deriva sus columnas, índices e invariantes para la migración AUTH-DB-02. El contrato no declara resueltas las brechas DB02-09.

El Anexo B sigue siendo la fuente de campos base y estados. Las ampliaciones del contrato incluyen `usuario.correo_original`, `sesion.ultima_actividad_en` y `token_accion.invalidado_en`, necesarias por las decisiones ratificadas; `usuario.intentos_fallidos_consecutivos` y `usuario.bloqueado_hasta` derivan de los requisitos existentes RQF-014/015/017. Estas adiciones no cambian los requisitos ni los estados base y se trazan explícitamente en este diseño.

## Tipos, claves e integridad

- PK UUID; instantes `timestamptz`; estados, propósitos y canales limitados por `CHECK` a los literales aprobados.
- Toda FK histórica usa `ON DELETE RESTRICT` o `NO ACTION`; no se borra en cascada una cuenta, aceptación, sesión o token referenciado.
- Las constraints `UNIQUE` son la defensa definitiva ante concurrencia. Las validaciones de aplicación no las sustituyen.
- Los tokens se almacenan solo como hash; la contraseña solo como hash adaptativo versionado conforme RNF-013. Nunca persistir secretos en claro ni incluirlos en logs/eventos/métricas.

| Tabla | PK y únicas | Columnas / restricciones relevantes |
| --- | --- | --- |
| `usuario` | PK (`id`); UNIQUE (`correo_normalizado`). | `correo_original` conserva el valor ingresado; `correo_normalizado` es la clave canónica derivada tras validar formato: eliminar espacios externos y aplicar Unicode `casefold` independiente del idioma a toda la dirección. Aplicar exactamente el mismo algoritmo en registro, login y recuperación. `hash_clave`, `estado`, `creado_en`, `actualizado_en`, `baja_solicitada_en` según Anexo B. `intentos_fallidos_consecutivos` NOT NULL DEFAULT 0 y `bloqueado_hasta` nullable persisten RQF-014/015/017. No eliminar puntos ni `+alias` de proveedores. |
| `rol_usuario` | PK (`usuario_id`, `rol`). | FK a `usuario.id` RESTRICT; `rol` limitado a `arrendatario`, `arrendador`, `administrador`; `concedido_por` nullable y FK RESTRICT. La preferencia RQF-213 no es un rol y no se almacena aquí. |
| `sesion` | PK (`id`); UNIQUE (`token_hash`). | FK `usuario_id` RESTRICT; `creada_en`; `ultima_actividad_en` NOT NULL, inicialmente igual a `creada_en`; `expira_en` como vencimiento absoluto `creada_en + 8 horas`; `revocada_en` nullable. Comprobar `ultima_actividad_en >= creada_en`, `expira_en > creada_en` y `expira_en <= creada_en + interval '8 hours'`. |
| `token_accion` | PK (`id`); UNIQUE (`token_hash`). | FK `usuario_id` RESTRICT; propósito solo `verificar_correo` o `recuperar_clave`; `creado_en`, `expira_en`, `consumido_en`, `invalidado_en`, `intentos` con `CHECK 0 <= intentos <= 5`. `invalidado_en` nullable registra la invalidación autorizada al reemitir y no se confunde con consumo exitoso; no permitir que ambos terminales estén definidos. No incluir `cambiar_correo` en esta migración: ES1 no define ese flujo. |
| `version_terminos` | PK (`id`); UNIQUE (`codigo`). | `tipo` según Anexo B; hash SHA-256 del texto exacto publicado; versión publicada inmutable. |
| `aceptacion_terminos` | PK (`id`); UNIQUE (`usuario_id`, `version_id`). | FK a usuario/versión RESTRICT; `canal` limitado a `web`, `api`, `administrado`; aceptación asociada a la versión vigente en el instante del acto. |

## Intentos de inicio de sesión (separados de límites de token)

`intentos_fallidos_consecutivos` persiste los fallos de login de RQF-014/015; `bloqueado_hasta` registra el vencimiento del bloqueo de 30 minutos de RQF-017. Al alcanzar cinco fallos consecutivos, Backend bloquea el login hasta ese instante y emite la alerta requerida por RQF-016. El contador de tokens (`token_accion.intentos`) y los límites de emisión/IP no escriben estas columnas, no establecen `bloqueado_hasta` y no bloquean el login.

## Correo, unicidad y duplicados

La aplicación valida el formato primero, conserva el original y deriva la clave conforme a la autorización M01: `correo_normalizado = UnicodeCaseFold(trim_espacios_externos(correo_original))`. El casefold abarca toda la dirección y es independiente del idioma. No se añade NFC/NFKC ni reglas de proveedores no ratificadas. Login y recuperación buscan por la misma clave canónica; alta concurrente equivalente debe fallar en UNIQUE (`correo_normalizado`).

Esta constraint resuelve la unicidad de identidad requerida por ES1 RQF-002/RQF-007 y ES2 B.2. Para el mensaje de registro, respetar CU-01 A1 aunque implique enumeración: rechazar el duplicado e informar que debe iniciar sesión o recuperar. Para recuperación, respuesta genérica aunque CU-05 A2 aún diga que no se puede continuar. Las dos discrepancias están expuestas en `planning/decisiones_m01_identidad_sesion.md`; no cambiar la línea base ES1 desde el diseño DB.

## Sesiones y expiración

`expira_en` es el instante absoluto persistido (`creada_en + 8 horas`), no un TTL deslizante. `ultima_actividad_en` se persiste por separado. El Backend consulta su propio reloj y rechaza cuando `now >= expira_en` o `now >= ultima_actividad_en + 30 minutos`, conforme RNF-016. Solo solicitudes autenticadas de operaciones de usuario actualizan `ultima_actividad_en`; health checks, polling automático, keepalives y renovación de tokens no la actualizan. Nunca permitir que una actualización rebase el máximo absoluto. Revocación se registra en `revocada_en` y se valida en cada autorización.

Los índices de consulta pueden incluir (`usuario_id`, `expira_en`) WHERE `revocada_en IS NULL`; el vencimiento por reloj se comprueba en la consulta/Backend y no se codifica con `now()` en un predicado de índice parcial.

## Tokens y límites ratificados

`token_accion` admite únicamente `verificar_correo` (TTL 24 horas) y `recuperar_clave` (TTL 15 minutos conforme RQF-021/CU-05). Los instantes de emisión/expiración se fijan al crear el token según su propósito. No se añade flujo de cambio de correo ni propósito `cambiar_correo`; si aparece un requisito futuro, la confirmación tendrá TTL 15 minutos y deberá entrar por cambio trazable.

Cada secreto es aleatorio, de un solo uso, persistido solo como hash único y nunca en logs. Al emitir otro token del mismo usuario y propósito, el Backend marca `invalidado_en` en los anteriores activos dentro de la misma operación; `consumido_en` queda reservado para el consumo exitoso. Un token es válido solo si no está consumido ni invalidado, su `expira_en` está en el futuro y el límite de fallos no se alcanzó. `intentos` cuenta fallos, se incrementa de manera atómica y no admite más de 5; llegado a 5, no aceptar intentos posteriores.

El límite de 3 emisiones/reenvíos por cuenta y propósito se cuenta sobre una ventana móvil de una hora por `creado_en`, incluyendo tokens ya consumidos, expirados o invalidados. El índice propuesto para esa comprobación es (`usuario_id`, `proposito`, `creado_en`). No purgar filas antes de que salgan de la ventana móvil; una retención mayor no se define aquí. Las decisiones de token e intentos no deben bloquear la cuenta ni alterar los contadores/estado de login. Aplicar además límites por IP en Backend; sus umbrales numéricos no fueron especificados y no se inventan en este contrato de DB.

Los índices propuestos para acciones son UNIQUE (`token_hash`), (`usuario_id`, `proposito`, `expira_en`) WHERE `consumido_en IS NULL AND invalidado_en IS NULL`, más el índice de emisiones (`usuario_id`, `proposito`, `creado_en`). No usar `now()` en predicados de índices parciales: la vigencia cambia con el reloj, no con una actualización de fila.

## Brechas DB02-09: alcance de esta migración

| ID / requisito | Efecto en AUTH-DB-02 | Límite y seguimiento |
| --- | --- | --- |
| DB02-09 / RQF-213 | Afecta la cobertura de persistencia del registro: el modelo de seis tablas no tiene columna de preferencia y `rol_usuario` no equivale a esa preferencia no excluyente. | No agregarla silenciosamente al esquema de seis tablas. Mantener la brecha abierta y trazada; antes de afirmar cobertura completa de RQF-213 se requiere ticket/decisión de modelo. |
| DB02-09 / RQF-217 | Afecta la persistencia que necesitaría CU-50 para rechazar credenciales usadas durante los últimos 3 meses; Anexo B no define historial. | No crear tabla/campos de historial en esta migración. Mantener abierta decisión de minimización/retención y ticket antes del DDL que la implemente. El esquema base no puede declararse como cobertura de RQF-217. |
| DB02-09 / RQF-218 | Afecta el contrato e integración del aviso de cambio de contraseña, no una columna de las seis tablas M01. Capacidades genéricas de notificación/outbox pertenecen a M09/M11. | Mantener abierto contrato del evento y ownership. No añadir una tabla duplicada de notificación/outbox ni declarar cumplida la notificación por la existencia de tablas comunes. |

**Conclusión de alcance:** las seis tablas base pueden diseñarse sin cerrar DB02-09 si el PR y la migración no afirman cubrir preferencia, historial ni notificación. RQF-213 y RQF-217 son brechas de cobertura de datos/credenciales que requieren seguimiento antes del DDL específico correspondiente; RQF-218 es una brecha de integración fuera de estas tablas. Las tres permanecen abiertas. Ver `planning/revision_diccionario_datos.md` (crosswalk RQF-213–218 y DB02-09) y `planning/decisiones_m01_identidad_sesion.md`.

## Verificaciones previstas para AUTH-DB-02

1. Migrar desde una base vacía con la versión fijada de PostgreSQL y el migrador aprobado; repetir mediante el mismo migrador y comprobar que no duplica ni altera versiones/checksums.
2. En PostgreSQL, comprobar el esquema y las propiedades persistibles: columnas/tipos previstos; FK RESTRICT; dominios de estado/rol/canal/propósito; rangos CHECK de `intentos` y `intentos_fallidos_consecutivos`; persistencia de `bloqueado_hasta` y de los tiempos de sesión; índices y unicidad. Estas pruebas no comprueban el bloqueo de login ni el comportamiento de expiración ejecutado por Backend.
3. Usar filas sintéticas para comprobar UNIQUE real sobre `correo_normalizado`, incluso con inserciones concurrentes, y que `correo_original` puede conservarse junto a la clave normalizada. La aplicación calcula la clave con el algoritmo ratificado; AUTH-DB-02 no prueba ese algoritmo ni los flujos de registro/login/recuperación.
4. Limitar las verificaciones de tokens aquí a persistencia: columnas/campos, UNIQUE del hash, propósito, instantes `creado_en`/`expira_en`, contador dentro del CHECK de 0–5, y persistencia separada de `consumido_en` e `invalidado_en` mediante fixtures sintéticos. No comprobar emisión, consumo efectivo, TTL aplicado por servicio, invalidación al reemitir, conteo por ventana móvil ni límites por IP.
5. Confirmar que las migraciones ya existentes cumplen el contrato del migrador antes de ejecutar la suite. No usar la base local persistente ni datos reales; destruir la instancia PostgreSQL temporal al finalizar y comprobar que fue eliminada.

### Verificaciones de comportamiento fuera de AUTH-DB-02

- **AUTH-BE-02 + AUTH-TEST-01:** con reloj controlado, probar expiración idle/absoluta en Backend: solo operaciones autenticadas de usuario renuevan actividad, actividades excluidas no lo hacen y ninguna renovación rebasa 8 horas.
- **AUTH-BE-02 (verificación) y AUTH-BE-03 (recuperación) + AUTH-TEST-01:** probar emisión/consumo/reemisión, TTL por propósito, invalidación de tokens previos, intentos fallidos, límites por cuenta/propósito y por IP; comprobar que estos límites no bloquean el login.

Este documento solo define el diseño y el plan de verificación; no crea tablas, migraciones, instancias ni tickets y no cambia estados de Kanban.
